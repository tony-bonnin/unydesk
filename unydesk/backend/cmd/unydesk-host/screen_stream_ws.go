package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func streamScreenFrames(ctx context.Context, done <-chan struct{}, sessionURL, screenWSURL, sessionID string) {
	pipeline := newScreenPipeline(defaultScreenPipelineProfile())

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
		}

		if err := runScreenStreamLoop(ctx, done, sessionURL, screenWSURL, sessionID, pipeline); err != nil {
			fmt.Printf("Host screen stream error for %s: %v\n", sessionID, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-time.After(750 * time.Millisecond):
		}
	}
}

func runScreenStreamLoop(ctx context.Context, done <-chan struct{}, sessionURL, screenWSURL, sessionID string, pipeline *screenPipeline) error {
	dialer := *websocket.DefaultDialer
	dialer.EnableCompression = false
	conn, _, err := dialer.Dial(screenWSURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	lastCursorKind := ""

	sendCaptureError := func(captureErr error) {
		friendly, shouldReport := pipeline.captureErrorMessage(captureErr)
		if !shouldReport {
			return
		}
		fmt.Printf("[%s] screen error: %v\n", time.Now().Format("15:04:05"), captureErr)
		_ = conn.WriteJSON(map[string]any{
			"type":  "screen_error",
			"error": friendly,
		})
	}
	sendCursorUpdate := func(force bool) error {
		nextCursorKind := normalizeRemoteCursorKind(detectHostCursorKind())
		if !force && nextCursorKind == lastCursorKind {
			return nil
		}
		lastCursorKind = nextCursorKind
		return conn.WriteJSON(map[string]any{
			"type":   "cursor",
			"cursor": nextCursorKind,
		})
	}

	if err := sendCursorUpdate(true); err != nil {
		return err
	}

	frame, changed, err := pipeline.nextFrame()
	if err != nil {
		sendCaptureError(err)
		time.Sleep(pipeline.profile.CaptureFailureBackoff)
	} else if changed {
		if writeErr := conn.WriteMessage(websocket.BinaryMessage, encodeScreenWireFrame(frame)); writeErr != nil {
			return writeErr
		}
	}

	frameTicker := time.NewTicker(pipeline.profile.FrameInterval)
	defer frameTicker.Stop()
	statusTicker := time.NewTicker(2 * time.Second)
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-statusTicker.C:
			snapshot, statusErr := fetchSessionSnapshot(sessionURL)
			if statusErr == nil && strings.EqualFold(strings.TrimSpace(snapshot.Status), "closed") {
				return nil
			}
		case <-frameTicker.C:
			if err := sendCursorUpdate(false); err != nil {
				return err
			}
			nextFrame, nextChanged, captureErr := pipeline.nextFrame()
			if captureErr != nil {
				sendCaptureError(captureErr)
				time.Sleep(pipeline.profile.CaptureFailureBackoff)
				continue
			}
			if !nextChanged {
				continue
			}
			if writeErr := conn.WriteMessage(websocket.BinaryMessage, encodeScreenWireFrame(nextFrame)); writeErr != nil {
				return writeErr
			}
		}
	}
}
