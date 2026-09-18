package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// backup streams a gzipped tar of the SQLite control-plane database (taken
// via VACUUM INTO for a WAL-consistent snapshot, never a raw copy of the
// live file, which could catch a torn write mid-checkpoint) plus the master
// encryption key. The two travel together deliberately -- see
// internal/secrets' package doc: losing the key file makes every secret
// (PATs, webhook secrets, secret env vars) permanently unrecoverable no
// matter how intact the database is, so a backup that only grabs the DB is
// not actually a usable backup. Owner-only: paired together, these two
// files are equivalent to every secret on the box. See docs/backup.md for
// the restore path.
func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	snapshotPath, err := s.snapshotDB(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot database: "+err.Error())
		return
	}
	defer os.Remove(snapshotPath)

	keyPath := filepath.Join(s.DataDir, "master.key")
	if _, err := os.Stat(keyPath); err != nil {
		writeError(w, http.StatusInternalServerError, "master key not found at "+keyPath+": "+err.Error())
		return
	}

	ts := time.Now().UTC()
	filename := fmt.Sprintf("mangrove-backup-%s.tar.gz", ts.Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)

	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	info := fmt.Sprintf(
		"Mangrove backup taken %s.\n\n"+
			"Contains:\n"+
			"  mangrove.db  -- control-plane database (consistent snapshot via VACUUM INTO)\n"+
			"  master.key   -- secrets encryption key; without it, every secret in\n"+
			"                  mangrove.db (PATs, webhook secrets, secret env vars) is\n"+
			"                  permanently unrecoverable, even with the database intact.\n\n"+
			"Restore: on a fresh box, before starting mangrove for the first time, extract\n"+
			"both files into MANGROVE_DATA_DIR (default ./data, /var/lib/mangrove in a\n"+
			"setup.sh install). See docs/backup.md.\n",
		ts.Format(time.RFC3339))

	if err := tarFile(tw, "BACKUP_INFO.txt", []byte(info), 0o644, ts); err != nil {
		return // headers already sent; nothing more we can do but stop writing
	}
	if err := tarFileFromDisk(tw, "mangrove.db", snapshotPath, 0o600); err != nil {
		return
	}
	if err := tarFileFromDisk(tw, "master.key", keyPath, 0o600); err != nil {
		return
	}
}

// snapshotDB writes a WAL-consistent copy of the live database to a fresh
// temp path via SQLite's VACUUM INTO (refuses to run if the target already
// exists, hence create-then-remove to reserve a unique name first) and
// returns that path. The caller owns cleanup.
func (s *Server) snapshotDB(ctx context.Context) (string, error) {
	tmp, err := os.CreateTemp("", "mangrove-backup-*.db")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	tmp.Close()
	os.Remove(path)

	if _, err := s.Store.DB.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", err
	}
	return path, nil
}

func tarFile(tw *tar.Writer, name string, data []byte, mode int64, modTime time.Time) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: modTime}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func tarFileFromDisk(tw *tar.Writer, name, path string, mode int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: stat.Size(), ModTime: stat.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
