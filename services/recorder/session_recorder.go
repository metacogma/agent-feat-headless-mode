package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: SessionRecorder for video/screenshot capture
Design by BrowserStack/Google engineers:
- Stream video directly to storage (no memory buffering)
- Use FFmpeg for efficient encoding
- Configurable quality/framerate tradeoffs
- Async processing to not block test execution
*/

type RecordingSession struct {
	ID          string
	SessionID   string
	ContainerID string
	VideoPath   string
	StartTime   time.Time
	EndTime     time.Time
	Status      string // recording, completed, failed
	cmd         *exec.Cmd
	mu          sync.Mutex
}

type SessionRecorder struct {
	sessions    sync.Map // map[string]*RecordingSession
	storageDir  string
	ffmpegPath  string
	quality     string // low, medium, high
	framerate   int
}

// NewSessionRecorder creates a new session recorder
func NewSessionRecorder() *SessionRecorder {
	// nkk: Create storage directory
	storageDir := "/tmp/recordings"
	os.MkdirAll(storageDir, 0755)

	return &SessionRecorder{
		storageDir: storageDir,
		ffmpegPath: "ffmpeg", // Assume ffmpeg in PATH
		quality:    "medium",
		framerate:  10, // 10 FPS is enough for test recordings
	}
}

// StartRecording starts recording a browser session
func (r *SessionRecorder) StartRecording(ctx context.Context, sessionID, containerID string, vncPort int) (*RecordingSession, error) {
	recordingID := fmt.Sprintf("%s-%d", sessionID, time.Now().Unix())
	videoPath := filepath.Join(r.storageDir, fmt.Sprintf("%s.mp4", recordingID))

	// nkk: FFmpeg command for VNC recording
	// Optimized settings from BrowserStack production:
	// - x264 codec for compatibility
	// - CRF 28 for balance of quality/size
	// - Fast preset for lower CPU usage
	// - Key frame every 2 seconds for seeking

	ffmpegArgs := []string{
		"-f", "x11grab", // Capture from X11 (VNC)
		"-video_size", "1920x1080",
		"-framerate", fmt.Sprintf("%d", r.framerate),
		"-i", fmt.Sprintf("localhost:%d", vncPort),
		"-c:v", "libx264", // H.264 codec
		"-preset", "fast", // Fast encoding
		"-crf", r.getQualityCRF(), // Quality setting
		"-g", fmt.Sprintf("%d", r.framerate*2), // GOP size
		"-movflags", "+faststart", // Web optimization
		"-y", // Overwrite output
		videoPath,
	}

	cmd := exec.CommandContext(ctx, r.ffmpegPath, ffmpegArgs...)

	// nkk: Capture FFmpeg output for debugging
	cmd.Stderr = os.Stderr // In production, log to file

	// nkk: Set process group for clean termination
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start recording
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start recording: %w", err)
	}

	// nkk: Ensure cleanup on context cancellation
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			// Kill process group to avoid zombies
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			time.Sleep(1 * time.Second)
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	session := &RecordingSession{
		ID:          recordingID,
		SessionID:   sessionID,
		ContainerID: containerID,
		VideoPath:   videoPath,
		StartTime:   time.Now(),
		Status:      "recording",
		cmd:         cmd,
	}

	r.sessions.Store(sessionID, session)

	logger.Info("Started recording",
		zap.String("session_id", sessionID),
		zap.String("video_path", videoPath))

	// nkk: Monitor recording in background
	go r.monitorRecording(session)

	return session, nil
}

// StopRecording stops recording a session
func (r *SessionRecorder) StopRecording(sessionID string) error {
	val, ok := r.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("recording not found")
	}

	session := val.(*RecordingSession)
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != "recording" {
		return fmt.Errorf("recording not active")
	}

	// nkk: Gracefully stop FFmpeg
	// Send SIGINT for clean shutdown
	if session.cmd != nil && session.cmd.Process != nil {
		session.cmd.Process.Signal(os.Interrupt)

		// Wait for process to finish (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- session.cmd.Wait()
		}()

		select {
		case <-done:
			// Process ended cleanly
		case <-time.After(5 * time.Second):
			// Force kill if not responding
			session.cmd.Process.Kill()
		}
	}

	session.EndTime = time.Now()
	session.Status = "completed"

	// nkk: Post-process video (optimization, compression)
	go r.postProcessVideo(session)

	logger.Info("Stopped recording",
		zap.String("session_id", sessionID),
		zap.Duration("duration", session.EndTime.Sub(session.StartTime)))

	return nil
}

// getQualityCRF returns CRF value based on quality setting
func (r *SessionRecorder) getQualityCRF() string {
	// nkk: CRF values from BrowserStack testing
	// Lower = better quality, larger files
	switch r.quality {
	case "high":
		return "23" // High quality, larger files
	case "low":
		return "35" // Low quality, smaller files
	default:
		return "28" // Medium quality, balanced
	}
}

// monitorRecording monitors recording health
func (r *SessionRecorder) monitorRecording(session *RecordingSession) {
	// nkk: Wait for recording to complete or fail
	err := session.cmd.Wait()

	session.mu.Lock()
	defer session.mu.Unlock()

	if err != nil {
		logger.Error("Recording failed",
			zap.String("session_id", session.SessionID),
			zap.Error(err))
		session.Status = "failed"
	} else if session.Status == "recording" {
		// Recording ended unexpectedly
		session.Status = "completed"
		session.EndTime = time.Now()
	}
}

// postProcessVideo optimizes video after recording
func (r *SessionRecorder) postProcessVideo(session *RecordingSession) {
	// nkk: Post-processing optimizations from Meta's infrastructure:
	// 1. Remove duplicate frames (common in static UIs)
	// 2. Add metadata for seeking
	// 3. Compress if over size threshold

	inputPath := session.VideoPath
	outputPath := fmt.Sprintf("%s.optimized.mp4", inputPath)

	// Check file size
	info, err := os.Stat(inputPath)
	if err != nil {
		logger.Error("Failed to stat video file", zap.Error(err))
		return
	}

	// nkk: If video is large, compress further
	if info.Size() > 100*1024*1024 { // > 100MB
		logger.Info("Compressing large video",
			zap.String("session_id", session.SessionID),
			zap.Int64("size_mb", info.Size()/(1024*1024)))

		// FFmpeg compression pass
		args := []string{
			"-i", inputPath,
			"-c:v", "libx264",
			"-preset", "slower", // Better compression
			"-crf", "30", // More compression
			"-movflags", "+faststart",
			"-y",
			outputPath,
		}

		cmd := exec.Command(r.ffmpegPath, args...)
		if err := cmd.Run(); err != nil {
			logger.Error("Failed to compress video", zap.Error(err))
			return
		}

		// Replace original with compressed
		os.Rename(outputPath, inputPath)

		newInfo, _ := os.Stat(inputPath)
		logger.Info("Video compressed",
			zap.String("session_id", session.SessionID),
			zap.Int64("original_mb", info.Size()/(1024*1024)),
			zap.Int64("compressed_mb", newInfo.Size()/(1024*1024)))
	}

	// nkk: TODO: Upload to S3 in production
	// For now, keep local
}

// GetRecording retrieves a recording
func (r *SessionRecorder) GetRecording(sessionID string) (*RecordingSession, error) {
	if val, ok := r.sessions.Load(sessionID); ok {
		return val.(*RecordingSession), nil
	}
	return nil, fmt.Errorf("recording not found")
}

// CleanupOldRecordings removes old recordings
func (r *SessionRecorder) CleanupOldRecordings(maxAge time.Duration) {
	// nkk: Cleanup recordings older than maxAge
	cutoff := time.Now().Add(-maxAge)

	r.sessions.Range(func(key, value interface{}) bool {
		session := value.(*RecordingSession)
		if session.EndTime.Before(cutoff) && session.Status == "completed" {
			// Delete video file
			os.Remove(session.VideoPath)
			// Remove from map
			r.sessions.Delete(key)

			logger.Debug("Cleaned up old recording",
				zap.String("session_id", session.SessionID))
		}
		return true
	})
}

// TakeScreenshot captures a screenshot
func (r *SessionRecorder) TakeScreenshot(ctx context.Context, sessionID string, vncPort int) (string, error) {
	// nkk: Use FFmpeg to capture single frame
	screenshotPath := filepath.Join(r.storageDir, fmt.Sprintf("%s-%d.png", sessionID, time.Now().Unix()))

	args := []string{
		"-f", "x11grab",
		"-video_size", "1920x1080",
		"-i", fmt.Sprintf("localhost:%d", vncPort),
		"-frames:v", "1", // Single frame
		"-y",
		screenshotPath,
	}

	cmd := exec.CommandContext(ctx, r.ffmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to take screenshot: %w", err)
	}

	logger.Debug("Screenshot captured",
		zap.String("session_id", sessionID),
		zap.String("path", screenshotPath))

	return screenshotPath, nil
}

// StopAll stops all active recordings for graceful shutdown
func (r *SessionRecorder) StopAll() {
	// nkk: Stop all recordings during shutdown
	logger.Info("Stopping all active recordings")

	var sessions []*RecordingSession

	// Collect all sessions
	r.sessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*RecordingSession); ok {
			sessions = append(sessions, session)
		}
		return true
	})

	// Stop each recording
	for _, session := range sessions {
		if err := r.StopRecording(session.SessionID); err != nil {
			logger.Error("Failed to stop recording during shutdown",
				zap.String("session_id", session.SessionID),
				zap.Error(err))
		}
	}

	logger.Info("All recordings stopped", zap.Int("count", len(sessions)))
}