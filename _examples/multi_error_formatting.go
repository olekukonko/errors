package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/olekukonko/errors"
)

func main() {
	// Configuration
	totalErrors := 1000
	sampleRate := 10 // 10%
	errorLimit := 50

	// Initialize with reproducible seed for demo purposes
	r := rand.New(rand.NewSource(42))
	start := time.Now()

	// Create MultiError with sampling
	multi := errors.NewMultiError(
		errors.WithSampling(uint32(sampleRate)),
		errors.WithLimit(errorLimit),
		errors.WithRand(r),
		errors.WithFormatter(createFormatter(totalErrors)),
	)

	// Generate errors
	for i := 0; i < totalErrors; i++ {
		multi.Add(errors.Newf("operation %d failed", i))
	}

	// Calculate statistics
	duration := time.Since(start)
	sampledCount := multi.Count()
	actualRate := float64(sampledCount) / float64(totalErrors) * 100

	// Print results
	fmt.Println(multi)
	printStatistics(totalErrors, sampledCount, sampleRate, actualRate, duration)
	printErrorDistribution(multi, 5) // Show top 5 errors
}

func createFormatter(total int) errors.ErrorFormatter {
	return func(errs []error) string {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Sampled Error Report (%d/%d):\n", len(errs), total))
		sb.WriteString("══════════════════════════════\n")
		return sb.String()
	}
}

func printStatistics(total, sampled, targetRate int, actualRate float64, duration time.Duration) {
	fmt.Printf("\nStatistics:\n")
	fmt.Printf("├─ Total errors generated: %d\n", total)
	fmt.Printf("├─ Errors captured: %d (limit: %d)\n", sampled, 50)
	fmt.Printf("├─ Target sampling rate: %d%%\n", targetRate)
	fmt.Printf("├─ Actual sampling rate: %.1f%%\n", actualRate)
	fmt.Printf("├─ Processing time: %v\n", duration)

	switch {
	case sampled == 50 && actualRate < float64(targetRate):
		fmt.Printf("└─ Note: Hit storage limit - actual rate would be ~%.1f%% without limit\n",
			float64(targetRate))
	case actualRate < float64(targetRate)*0.8 || actualRate > float64(targetRate)*1.2:
		fmt.Printf("└─ ⚠️ Warning: Significant sampling deviation\n")
	default:
		fmt.Printf("└─ Sampling within expected range\n")
	}
}

func printErrorDistribution(m *errors.MultiError, maxDisplay int) {
	errs := m.Errors()
	if len(errs) == 0 {
		return
	}

	fmt.Printf("\nError Distribution (showing first %d):\n", maxDisplay)
	for i, err := range errs {
		if i >= maxDisplay {
			fmt.Printf("└─ ... and %d more\n", len(errs)-maxDisplay)
			break
		}
		fmt.Printf("%s %v\n", getProgressBar(i, len(errs)), err)
	}
}

func getProgressBar(index, total int) string {
	const width = 10
	pos := int(float64(index) / float64(total) * width)
	return fmt.Sprintf("├─%s%s┤", strings.Repeat("■", pos), strings.Repeat(" ", width-pos))
}
