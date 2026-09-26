package ephemeral

import (
	"runtime"

	"azaman-engine/internal/models"
)

// ScrubBytes overwrites a byte slice with zeros to wipe sensitive financial data from RAM.
func ScrubBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ScrubRawTransactions zeroes raw lines and unmasked data in raw transactions.
func ScrubRawTransactions(txs []models.RawTransaction) {
	for i := range txs {
		txs[i].Description = ""
		txs[i].RawLine = ""
		txs[i].Reference = ""
	}
}

// ForceEphemeralCleanup triggers garbage collection to purge ephemeral statement buffers immediately.
func ForceEphemeralCleanup() {
	runtime.GC()
}
