package main

import "time"

type screenPipelineProfile struct {
	StreamMaxEdge         int
	SignatureColumns      int
	SignatureRows         int
	RefreshUnchangedAfter time.Duration
	FrameInterval         time.Duration
	CaptureRetryDelay     time.Duration
	CaptureFailureBackoff time.Duration
	CaptureErrorCooldown  time.Duration
	KeyframeInterval      time.Duration
	DiffTileSize          int
	PatchAreaThreshold    float64
	Codec                 screenCodecProfile
}

type screenCodecProfile struct {
	RawKeyframeBudgetBytes      int
	RawPatchBudgetBytes         int
	LosslessKeyframeMaxPixels   int
	LosslessKeyframeBudgetBytes int
	LosslessPatchMaxPixels      int
	LosslessPatchBudgetBytes    int
	JPEGQualityBase             int
	JPEGQualitySoft             int
	JPEGQualityHard             int
	JPEGQualityFloor            int
	JPEGSoftBudgetBytes         int
	JPEGHardBudgetBytes         int
	JPEGFloorBudgetBytes        int
	WebPQualityBase             float32
	WebPQualitySoft             float32
	WebPQualityHard             float32
	WebPQualityFloor            float32
	WebPSoftBudgetBytes         int
	WebPHardBudgetBytes         int
	WebPFloorBudgetBytes        int
}

func defaultScreenPipelineProfile() screenPipelineProfile {
	return screenPipelineProfile{
		StreamMaxEdge:         4096,
		SignatureColumns:      32,
		SignatureRows:         18,
		RefreshUnchangedAfter: time.Second,
		FrameInterval:         25 * time.Millisecond,
		CaptureRetryDelay:     35 * time.Millisecond,
		CaptureFailureBackoff: 250 * time.Millisecond,
		CaptureErrorCooldown:  2 * time.Second,
		KeyframeInterval:      1200 * time.Millisecond,
		DiffTileSize:          12,
		PatchAreaThreshold:    0.82,
		Codec: screenCodecProfile{
			RawKeyframeBudgetBytes:      0,
			RawPatchBudgetBytes:         1536 * 1024,
			LosslessKeyframeMaxPixels:   4096 * 2304,
			LosslessKeyframeBudgetBytes: 3 * 1024 * 1024,
			LosslessPatchMaxPixels:      1400 * 1400,
			LosslessPatchBudgetBytes:    2 * 1024 * 1024,
			JPEGQualityBase:             96,
			JPEGQualitySoft:             93,
			JPEGQualityHard:             90,
			JPEGQualityFloor:            86,
			JPEGSoftBudgetBytes:         2 * 1024 * 1024,
			JPEGHardBudgetBytes:         3 * 1024 * 1024,
			JPEGFloorBudgetBytes:        5 * 1024 * 1024,
			WebPQualityBase:             100,
			WebPQualitySoft:             98,
			WebPQualityHard:             96,
			WebPQualityFloor:            92,
			WebPSoftBudgetBytes:         1536 * 1024,
			WebPHardBudgetBytes:         2560 * 1024,
			WebPFloorBudgetBytes:        4 * 1024 * 1024,
		},
	}
}
