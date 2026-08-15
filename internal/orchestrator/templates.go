package orchestrator

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/templates"
)

// InstalledDeployment reports what InstallTemplate did with one of a
// template's deployments.
type InstalledDeployment struct {
	DeploymentID int64  `json:"deployment_id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	// DeployError is set if this deployment's rows were created but the
	// deploy itself failed (e.g. admission control, a bad image ref). Rows
	// created before the failure are left in place -- same as any other
	// failed deploy -- rather than silently rolled back.
	DeployError string `json:"deploy_error,omitempty"`
}

// TemplateInstallResult is what a template install returns: what got
// created, and any credentials generated along the way. Credentials are
// plaintext and returned exactly once -- they're stored encrypted (see
// api/env.go's existing write-only secret design) and never retrievable
// through the API again after this response.
type TemplateInstallResult struct {
	TemplateKey string                `json:"template_key"`
	Deployments []InstalledDeployment `json:"deployments"`
	Credentials map[string]string     `json:"credentials,omitempty"`
}

var (
	aliasPlaceholderRe     = regexp.MustCompile(`\{\{alias:([^}]+)\}\}`)
	generatedPlaceholderRe = regexp.MustCompile(`\{\{generated:([^}]+)\}\}`)
)

// InstallTemplate expands a built-in template into real deployment/
// service/volume/env rows -- exactly the same rows a user creates by hand
// through the normal API -- and then deploys each one in order through the
// existing Deploy(), so a template install goes through the same admission
// control, port registry, and volume wiring as any other deployment. A
// linked dependency (e.g. WordPress's MySQL) is created and deployed
// first, so its stable network alias and any generated credentials it
// produced exist before the deployment that references them is created. A
// "dockerfile"-strategy deployment builds from its GitURL/GitBranch the
// same way a hand-created git-backed deployment does, with no AuthToken --
// only public repos are reachable this way.
//
// envOverrides supplies values for the template's Prompt env vars, keyed by
// slug_suffix then env key (the same shape memoryOverridesMB uses for
// per-deployment memory) -- these come from the caller (the install form)
// and are substituted in at install time, never stored as part of the
// template itself. Every Required prompt var across the whole template
// must have a non-empty override before anything is created, so a missing
// value fails fast rather than leaving a half-installed template behind.
//
// If a deployment's rows fail to create, installation stops immediately
// (nothing useful can follow). If a deployment's rows are created but its
// Deploy() call fails, installation also stops there -- deployments
// created and deployed so far are left running (not rolled back), and the
// failure is reported in the result for whichever deployment hit it.
func (o *Orchestrator) InstallTemplate(ctx context.Context, projectID int64, templateKey, baseSlug string, memoryOverridesMB map[string]int, envOverrides map[string]map[string]string) (TemplateInstallResult, error) {
	tpl, ok := templates.Get(templateKey)
	if !ok {
		return TemplateInstallResult{}, fmt.Errorf("unknown template %q", templateKey)
	}
	if baseSlug == "" {
		return TemplateInstallResult{}, fmt.Errorf("slug is required")
	}
	for _, d := range tpl.Deployments {
		for _, ev := range d.Env {
			if ev.Prompt && ev.Required && envOverrides[d.SlugSuffix][ev.Key] == "" {
				return TemplateInstallResult{}, fmt.Errorf("missing required value for %s", ev.Key)
			}
		}
	}

	result := TemplateInstallResult{TemplateKey: templateKey, Credentials: map[string]string{}}
	generatedValues := map[string]string{} // generate_key -> plaintext
	aliasBySuffix := map[string]string{}   // slug_suffix -> stable container/network-alias name

	for _, d := range tpl.Deployments {
		slug := baseSlug + d.SlugSuffix
		name := tpl.Name + d.NameSuffix

		buildStrategy := d.BuildStrategy
		if buildStrategy == "" {
			buildStrategy = "image"
		}

		dep, err := o.Store.CreateDeployment(ctx, store.CreateDeploymentParams{
			ProjectID:     projectID,
			Name:          name,
			Slug:          slug,
			BuildStrategy: buildStrategy,
			GitBranch:     d.GitBranch,
			ImageRef:      d.ImageRef,
			RootPath:      ".",
		})
		if err != nil {
			return result, fmt.Errorf("create deployment %q: %w", slug, err)
		}

		memoryMB := d.MemoryLimitMB
		if override, ok := memoryOverridesMB[d.SlugSuffix]; ok && override > 0 {
			memoryMB = override
		}

		const serviceName = "app"
		containerName := fmt.Sprintf("mangrove-%s-%s", slug, serviceName)
		svc, err := o.Store.CreateService(ctx, store.CreateServiceParams{
			DeploymentID:    dep.ID,
			Name:            serviceName,
			ContainerName:   containerName,
			InternalPort:    d.InternalPort,
			IsInternalOnly:  true, // every template starts internal-only, same default as a hand-created deployment; user opts into public afterward
			CPULimitCores:   d.CPULimitCores,
			MemoryLimitMB:   memoryMB,
			HealthCheckPath: d.HealthCheckPath,
			Command:         d.Command,
		})
		if err != nil {
			return result, fmt.Errorf("create service for %q: %w", slug, err)
		}

		for _, vol := range d.Volumes {
			dockerVolumeName := fmt.Sprintf("mangrove-%s-%s-%s", slug, serviceName, vol.Name)
			svcID := svc.ID
			if _, err := o.Store.CreateVolume(ctx, store.CreateVolumeParams{
				DeploymentID:     dep.ID,
				ServiceID:        &svcID,
				Name:             vol.Name,
				DockerVolumeName: dockerVolumeName,
				MountPath:        vol.MountPath,
			}); err != nil {
				return result, fmt.Errorf("create volume %q for %q: %w", vol.Name, slug, err)
			}
		}

		for _, ev := range d.Env {
			var value string
			switch {
			case ev.Prompt:
				// Required-and-missing already failed fast above; an
				// optional prompt var with no override installs as empty.
				value = envOverrides[d.SlugSuffix][ev.Key]
			case ev.Generate != "":
				value, err = generateValue(ev.Generate, generatedValues)
				if err != nil {
					return result, fmt.Errorf("generate value for %s on %q: %w", ev.Key, slug, err)
				}
				if ev.GenerateKey != "" {
					generatedValues[ev.GenerateKey] = value
				}
				result.Credentials[fmt.Sprintf("%s: %s", name, ev.Key)] = value
			default:
				value, err = resolvePlaceholders(ev.Value, slug, baseSlug, aliasBySuffix, generatedValues, o.Config.BaseDomain)
				if err != nil {
					return result, fmt.Errorf("resolve value for %s on %q: %w", ev.Key, slug, err)
				}
			}

			if ev.Secret {
				aad := []byte(fmt.Sprintf("env_vars:%d:%s", svc.ID, ev.Key))
				ciphertext, nonce, err := o.Secrets.Seal(aad, []byte(value))
				if err != nil {
					return result, fmt.Errorf("encrypt %s for %q: %w", ev.Key, slug, err)
				}
				if err := o.Store.CreateSecretEnvVar(ctx, svc.ID, ev.Key, ciphertext, nonce); err != nil {
					return result, fmt.Errorf("save secret %s for %q: %w", ev.Key, slug, err)
				}
			} else {
				if err := o.Store.CreatePlainEnvVar(ctx, svc.ID, ev.Key, value); err != nil {
					return result, fmt.Errorf("save env %s for %q: %w", ev.Key, slug, err)
				}
			}
		}

		aliasBySuffix[d.SlugSuffix] = containerName

		var fileMounts []executor.FileMount
		for _, f := range d.Files {
			content, err := resolvePlaceholders(f.Content, slug, baseSlug, aliasBySuffix, generatedValues, o.Config.BaseDomain)
			if err != nil {
				return result, fmt.Errorf("resolve file %q for %q: %w", f.Path, slug, err)
			}
			fileMounts = append(fileMounts, executor.FileMount{Path: f.Path, Content: []byte(content)})
		}

		deployReq := DeployRequest{DeploymentID: dep.ID, TriggeredBy: "api", Files: fileMounts}
		if buildStrategy != "image" {
			// No AuthToken: template repos are built as unauthenticated
			// public clones (see Deployment's doc comment).
			deployReq.GitURL = d.GitURL
			deployReq.GitRef = d.GitBranch
		}

		installed := InstalledDeployment{DeploymentID: dep.ID, Slug: slug, Name: name}
		if _, deployErr := o.Deploy(ctx, deployReq); deployErr != nil {
			installed.DeployError = deployErr.Error()
			result.Deployments = append(result.Deployments, installed)
			return result, fmt.Errorf("deploy %q: %w", slug, deployErr)
		}
		result.Deployments = append(result.Deployments, installed)
	}

	return result, nil
}

// resolvePlaceholders expands {{slug}}, {{base_slug}}, {{base_domain}},
// {{alias:suffix}}, and {{generated:key}} inside an env var's literal value
// template or a template deployment's inline file content.
//
// {{base_slug}} is the user's chosen base slug (the template's primary
// deployment slug) regardless of which deployment the placeholder appears
// on -- it's how a dependency whose slug includes a suffix (e.g. a Supabase
// Studio deployment at "<base>-studio") can still reference the primary
// deployment's public URL.
func resolvePlaceholders(value, ownSlug, baseSlug string, aliasBySuffix, generatedValues map[string]string, baseDomain string) (string, error) {
	value = strings.ReplaceAll(value, "{{slug}}", ownSlug)
	value = strings.ReplaceAll(value, "{{base_slug}}", baseSlug)
	value = strings.ReplaceAll(value, "{{base_domain}}", baseDomain)

	for _, m := range aliasPlaceholderRe.FindAllStringSubmatch(value, -1) {
		alias, ok := aliasBySuffix[m[1]]
		if !ok {
			return "", fmt.Errorf("no deployment with slug_suffix %q created yet for {{alias:%s}}", m[1], m[1])
		}
		value = strings.ReplaceAll(value, m[0], alias)
	}
	for _, m := range generatedPlaceholderRe.FindAllStringSubmatch(value, -1) {
		gv, ok := generatedValues[m[1]]
		if !ok {
			return "", fmt.Errorf("no generated value for key %q yet", m[1])
		}
		value = strings.ReplaceAll(value, m[0], gv)
	}
	return value, nil
}

const alnumCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// generateValue produces a random credential. Kinds:
//
//   - "password": alnum-only 24 chars, so it's always safe to embed directly
//     in a connection-string URL or a shell command without escaping.
//   - "hex32": 32 lowercase hex chars (16 random bytes), for things that
//     require exactly 32 characters (e.g. a Postgres-meta/Studio crypto key).
//   - "hex64": 64 lowercase hex chars (32 random bytes), for HS256 JWT
//     secrets / Elixir secret-key-bases that want at least 32 bytes.
//   - "jwt:<role>:{{generated:<secret_key>}}": an HS256-signed Supabase API
//     key (anon or service_role) whose signature uses the referenced,
//     earlier-generated secret -- see generateSupabaseJWT.
//
// generatedValues lets a jwt generate resolve its {{generated:...}} secret
// reference the same way a literal value would.
func generateValue(kind string, generatedValues map[string]string) (string, error) {
	switch {
	case kind == "password":
		return randomAlnum(24)
	case kind == "hex32":
		return randomHex(16)
	case kind == "hex64":
		return randomHex(32)
	case strings.HasPrefix(kind, "jwt:"):
		return generateSupabaseJWT(kind, generatedValues)
	default:
		return "", fmt.Errorf("unknown generate kind %q", kind)
	}
}

// generateSupabaseJWT mints a Supabase API key as an HS256 JWT signed with
// the shared JWT_SECRET: header.role is what PostgREST/GoTrue use to `SET
// ROLE`, and the signature proves the key was created with the same secret
// every service in the template verifies against. The expiry is ~1000 years
// out on purpose -- these are long-lived static keys the user pastes into a
// client SDK, not user session tokens, so a short exp would silently break
// every anonymous request after the first hour.
func generateSupabaseJWT(kind string, generatedValues map[string]string) (string, error) {
	rest := strings.TrimPrefix(kind, "jwt:")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid jwt generate kind %q (want jwt:<role>:{{generated:<key>}})", kind)
	}
	role, ref := parts[0], parts[1]
	if role != "anon" && role != "service_role" {
		return "", fmt.Errorf("invalid jwt role %q (want anon or service_role)", role)
	}
	m := generatedPlaceholderRe.FindStringSubmatch(ref)
	if len(m) != 2 {
		return "", fmt.Errorf("jwt generate kind %q must reference a secret via {{generated:<key>}}", kind)
	}
	secret, ok := generatedValues[m[1]]
	if !ok {
		return "", fmt.Errorf("jwt generate kind %q references unknown generated secret %q", kind, m[1])
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	payloadJSON := fmt.Sprintf(`{"role":%q,"iss":"supabase","iat":%d,"exp":%d}`, role, now, now+31557600000)
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func randomAlnum(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alnumCharset))))
		if err != nil {
			return "", err
		}
		b[i] = alnumCharset[idx.Int64()]
	}
	return string(b), nil
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
