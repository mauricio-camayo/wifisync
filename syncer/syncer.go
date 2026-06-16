package syncer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"wifisync/config"
)

// ProgressEvent is emitted for each file successfully copied.
type ProgressEvent struct {
	File  string
	Bytes int64
}

// Result summarises a completed or cancelled sync run.
type Result struct {
	FilesCopied      int
	FilesSkipped     int
	BytesTransferred int64
	Duration         time.Duration
	Cancelled        bool
	Errors           []string
}

type Syncer struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Syncer {
	return &Syncer{cfg: cfg}
}

// Run performs a unidirectional sync from cfg.SourceFolder to cfg.DestFolder.
// Progress events are sent to the progress channel (may be nil).
// LastSyncTime on network is updated only on clean (non-cancelled, error-free) completion.
func (s *Syncer) Run(ctx context.Context, network *config.NetworkEntry, progress chan<- ProgressEvent) (Result, error) {
	start := time.Now()
	var result Result

	s.cfg.RLock()
	srcFolder := s.cfg.SourceFolder
	dstFolder := s.cfg.DestFolder
	timeout := time.Duration(s.cfg.PerFileCopyTimeoutMins) * time.Minute
	s.cfg.RUnlock()

	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	if _, err := os.Stat(srcFolder); err != nil {
		return result, fmt.Errorf("source folder not accessible: %w", err)
	}
	if _, err := os.Stat(dstFolder); err != nil {
		return result, fmt.Errorf("destination folder not accessible: %w", err)
	}

	cancelled := false

	walkErr := filepath.WalkDir(srcFolder, func(srcPath string, d os.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", srcPath, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			result.FilesSkipped++
			return nil
		}
		if ctx.Err() != nil {
			cancelled = true
			return filepath.SkipAll
		}

		rel, err := filepath.Rel(srcFolder, srcPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", srcPath, err))
			return nil
		}
		dstPath := filepath.Join(dstFolder, rel)

		srcInfo, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", rel, err))
			return nil
		}
		if !needsCopy(srcInfo, dstPath) {
			result.FilesSkipped++
			return nil
		}

		fileCtx, cancel := context.WithTimeout(ctx, timeout)
		n, copyErr := copyFile(fileCtx, srcPath, dstPath, srcInfo.ModTime())
		cancel()

		result.BytesTransferred += n

		if copyErr != nil {
			if fileCtx.Err() != nil {
				// Per-file timeout or parent context cancelled — stop the sync.
				cancelled = true
				return filepath.SkipAll
			}
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", rel, copyErr))
			return nil
		}

		result.FilesCopied++
		if progress != nil {
			select {
			case progress <- ProgressEvent{File: rel, Bytes: n}:
			default:
			}
		}
		return nil
	})

	result.Duration = time.Since(start)
	result.Cancelled = cancelled || ctx.Err() != nil

	if walkErr != nil {
		return result, walkErr
	}

	// Only mark the network synced on a clean run.
	if !result.Cancelled && len(result.Errors) == 0 {
		s.cfg.Lock()
		network.LastSyncTime = time.Now()
		s.cfg.Unlock()
	}

	return result, nil
}

// needsCopy returns true if the source file should be copied to dst.
// Copies when destination is missing, source is newer, or sizes differ.
func needsCopy(srcInfo os.FileInfo, dstPath string) bool {
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		return true
	}
	return srcInfo.ModTime().After(dstInfo.ModTime()) || srcInfo.Size() != dstInfo.Size()
}

// copyFile copies src to dst via a temp file, preserving the source mtime.
// The original dst is left untouched if the copy fails or times out.
func copyFile(ctx context.Context, src, dst string, mtime time.Time) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".wifisync-*.tmp")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()

	n, err := copyWithContext(ctx, tmp, srcFile)
	tmp.Close()

	if err != nil {
		os.Remove(tmpName)
		return n, err
	}

	os.Chtimes(tmpName, time.Now(), mtime)

	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return n, err
	}

	return n, nil
}

// copyWithContext copies from src to dst in 32 KB chunks, checking ctx before
// each chunk so timeouts and cancellations are respected promptly.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			total += int64(nw)
			if werr != nil {
				return total, werr
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}
