package explorer

import "sort"

// queueBandMins are the display fee bands of the mempool queue, front
// (highest fee) first. The design fixes these five: 15+, 10–15, 6–10, 4–6,
// 1–4 sat/vB; anything below 1 is lumped into the last band.
var queueBandMins = []float64{15, 10, 6, 4, 1}

// QueueBand is the vbytes waiting in one fee band. MaxSatPerVb is 0 for the
// open-ended front band.
type QueueBand struct {
	MinSatPerVb float64 `json:"minSatPerVb"`
	MaxSatPerVb float64 `json:"maxSatPerVb"`
	VBytes      int64   `json:"vbytes"`
}

// Queue is the mempool "line" visualization payload: fee-band histogram
// plus the next-block cutoff, all proportional to vbytes.
type Queue struct {
	TxCount     int         `json:"txCount"`
	TotalVBytes int64       `json:"totalVbytes"`
	Bands       []QueueBand `json:"bands"`

	// CutoffFraction is the next-block cutoff position along the bar:
	// one block's worth of vbytes from the front, as a fraction of the
	// total (1 when the whole mempool fits in the next block).
	CutoffFraction float64 `json:"cutoffFraction"`

	// NextBlockRate is the lowest feerate still inside the cutoff — pay
	// at least this and you likely make the next block.
	NextBlockRate float64 `json:"nextBlockRate"`
}

// queueFromSnapshot builds the histogram in one front-to-back walk over
// the entries sorted by feerate. An empty mempool yields empty bands,
// cutoff 1, and a 1 sat/vB floor rate.
func queueFromSnapshot(snapshot map[string]MempoolEntry) *Queue {
	q := &Queue{
		TxCount:        len(snapshot),
		Bands:          make([]QueueBand, len(queueBandMins)),
		CutoffFraction: 1,
		NextBlockRate:  1,
	}
	for i, lo := range queueBandMins {
		q.Bands[i].MinSatPerVb = lo
		if i > 0 {
			q.Bands[i].MaxSatPerVb = queueBandMins[i-1]
		}
	}
	if len(snapshot) == 0 {
		return q
	}

	type entry struct {
		rate  float64
		vsize int64
	}
	entries := make([]entry, 0, len(snapshot))
	for _, e := range snapshot {
		entries = append(entries, entry{rate: e.FeeRate, vsize: e.VSize})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].rate > entries[j].rate
	})

	// Front of the line first: rates only fall, so the band index only
	// advances. The cutoff is where one block of vbytes has been consumed.
	band := 0
	var cum int64
	cutoffRate := entries[len(entries)-1].rate
	cutoffSeen := false
	for _, e := range entries {
		for band < len(q.Bands)-1 && e.rate < q.Bands[band].MinSatPerVb {
			band++
		}
		q.Bands[band].VBytes += e.vsize
		q.TotalVBytes += e.vsize

		cum += e.vsize
		if !cutoffSeen && cum >= vbytesPerBlock {
			cutoffRate = e.rate
			cutoffSeen = true
		}
	}

	q.NextBlockRate = cutoffRate
	q.CutoffFraction = min(float64(vbytesPerBlock)/float64(q.TotalVBytes), 1)
	return q
}
