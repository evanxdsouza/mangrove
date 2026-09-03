package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

// registerTools wires every MCP tool to client. Each handler is a thin
// wrapper: call the apiclient method, return its result and error
// straight through. Returning (nil, out, err) rather than building a
// *mcp.CallToolResult by hand works because mcp.AddTool's generic wrapper
// already does the right thing with that pair -- marshals out into
// StructuredContent (and a TextContent fallback) on success, or wraps err
// into an IsError tool result on failure (see toolForErr in the SDK) --
// see docs/architecture.md for why nothing here builds one manually.
func registerTools(server *mcp.Server, client *apiclient.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List every workspace (an organizational grouping of projects, e.g. production vs staging) along with how many projects each has.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		out, err := client.ListWorkspaces(ctx)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_projects",
		Description: "List projects. Optionally filter to a single workspace.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		WorkspaceID *int64 `json:"workspace_id,omitempty" jsonschema:"only list projects in this workspace"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.ListProjects(ctx, args.WorkspaceID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_project",
		Description: "Get one project by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		ProjectID int64 `json:"project_id" jsonschema:"the project id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.GetProject(ctx, args.ProjectID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_deployments",
		Description: "List every deployment in a project, including staging environments and PR previews (see the environment field: production, staging, or preview).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		ProjectID int64 `json:"project_id" jsonschema:"the project id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.ListDeployments(ctx, args.ProjectID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_deployment",
		Description: "Get one deployment by id: build strategy, status, replica count, whether it's public, GitHub auto-deploy config, and more.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.GetDeployment(ctx, args.DeploymentID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_services",
		Description: "List the services (containers) that make up a deployment -- more than one for a compose stack or a linked template like WordPress+MySQL.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.ListServices(ctx, args.DeploymentID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_service",
		Description: "Get one service by id: its current container, image tag, resource limits, health check config.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		ServiceID int64 `json:"service_id" jsonschema:"the service id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.GetService(ctx, args.ServiceID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_deploy_history",
		Description: "List a deployment's deploy history (most recent first) -- what to pick a deploy_history_id from for rollback.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.ListDeployHistory(ctx, args.DeploymentID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "redeploy",
		Description: "Re-run the full build+deploy pipeline for a deployment against whatever source it's already configured with " +
			"(fresh build from the linked repo's current branch tip, or the same image ref for the image strategy). " +
			"Goes through the normal health-check-gated blue/green swap -- a failed deploy never takes down the currently-running one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id to redeploy"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.Redeploy(ctx, args.DeploymentID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "rollback",
		Description: "Roll back to a previous deploy_history entry (from list_deploy_history) by re-running the exact same build artifact " +
			"through the same health-check-gated blue/green swap as a forward deploy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeployHistoryID int64 `json:"deploy_history_id" jsonschema:"the deploy_history id to roll back to, from list_deploy_history"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.Rollback(ctx, args.DeployHistoryID)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stop_deployment",
		Description: "Stop a deployment's container(s) without removing them or its data -- a later restart is fast. Also frees its memory budget for other deployments.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id to stop"`
	}) (*mcp.CallToolResult, any, error) {
		err := client.Stop(ctx, args.DeploymentID)
		return nil, map[string]bool{"stopped": err == nil}, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "restart_deployment",
		Description: "Restart a deployment's container(s) in place (same container, same volumes) -- also how a stopped deployment is started back up. Not health-check-gated, unlike redeploy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id to restart"`
	}) (*mcp.CallToolResult, any, error) {
		err := client.Restart(ctx, args.DeploymentID)
		return nil, map[string]bool{"restarted": err == nil}, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "scale_deployment",
		Description: "Set a deployment's replica count (1-32) and redeploy so it takes effect. Only single-container build strategies " +
			"(dockerfile/nixpacks/image) support more than 1 replica; compose stacks and static sites stay at 1.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		DeploymentID int64 `json:"deployment_id" jsonschema:"the deployment id to scale"`
		Replicas     int   `json:"replicas" jsonschema:"desired replica count, 1-32"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.Scale(ctx, args.DeploymentID, args.Replicas)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "run_command",
		Description: "Run a one-off command inside a service's currently-running container (e.g. a database migration) via `docker exec`, " +
			"synchronously, returning combined stdout+stderr and the exit code. The container must be running. Not for long-running or " +
			"interactive processes -- there's no way to interrupt or attach to this from here once it starts.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		ServiceID int64    `json:"service_id" jsonschema:"the service id whose container to run this in"`
		Command   []string `json:"command" jsonschema:"argv, not a shell string -- e.g. [\"sh\",\"-c\",\"npm run migrate\"] to invoke a shell yourself"`
	}) (*mcp.CallToolResult, any, error) {
		out, err := client.RunCommand(ctx, args.ServiceID, args.Command)
		return nil, out, apiErrorText(err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_logs",
		Description: "Fetch a bounded tail of a service's container log output -- a snapshot (most recent lines), not a live stream.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args struct {
		ServiceID int64 `json:"service_id" jsonschema:"the service id to fetch logs for"`
		TailLines int   `json:"tail_lines,omitempty" jsonschema:"how many of the most recent lines to fetch (default 200)"`
	}) (*mcp.CallToolResult, any, error) {
		lines, err := client.Logs(ctx, args.ServiceID, args.TailLines)
		return nil, map[string]any{"lines": lines}, apiErrorText(err)
	})
}
