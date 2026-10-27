package slm

import (
	"math"
)

// --- Matrix operations (all flat []float32, row-major) ---

// matMul computes C = A × B where A is [m×k], B is [k×n].
// All slices are flat row-major: A[m*k], B[k*n], C[m*n].
func matMul(A []float32, B []float32, C []float32, m, k, n int) {
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			var sum float32
			for t := 0; t < k; t++ {
				sum += A[i*k+t] * B[t*n+j]
			}
			C[i*n+j] = sum
		}
	}
}

// matMulAdd computes C = A × B + C (fused multiply-add).
func matMulAdd(A []float32, B []float32, C []float32, m, k, n int) {
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			var sum float32
			for t := 0; t < k; t++ {
				sum += A[i*k+t] * B[t*n+j]
			}
			C[i*n+j] += sum
		}
	}
}

// addVectors adds B to A in-place: A[i] += B[i] for len(A) elements.
// B is broadcast if len(B) < len(A) (used for bias).
func addVectors(A []float32, B []float32) {
	if len(B) == len(A) {
		for i := range A {
			A[i] += B[i]
		}
	} else if len(B) == 1 {
		b := B[0]
		for i := range A {
			A[i] += b
		}
	} else {
		// Broadcast B over rows of A.
		n := len(B)
		for i := range A {
			A[i] += B[i%n]
		}
	}
}

// layerNorm computes Layer Normalization: y = (x - mean) / sqrt(var + eps) * gamma + beta.
func layerNorm(x []float32, gamma []float32, beta []float32, eps float32) {
	n := len(x)
	var mean, variance float32
	for _, v := range x {
		mean += v
	}
	mean /= float32(n)
	for _, v := range x {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float32(n)
	std := float32(math.Sqrt(float64(variance + eps)))

	for i := range x {
		x[i] = (x[i]-mean)/std*gamma[i] + beta[i]
	}
}

// gelu computes GELU activation in-place: x * 0.5 * (1 + erf(x / sqrt(2))).
func gelu(x []float32) {
	for i := range x {
		x[i] = x[i] * float32(0.5*(1+math.Erf(float64(x[i])/math.Sqrt2)))
	}
}

// relu computes ReLU activation in-place: max(0, x).
func relu(x []float32) {
	for i := range x {
		if x[i] < 0 {
			x[i] = 0
		}
	}
}

// softmax computes softmax in-place over the last `dim` elements.
// If x is [rows×dim], softmax is applied per row.
func softmax(x []float32, dim int) {
	rows := len(x) / dim
	for r := 0; r < rows; r++ {
		start := r * dim
		// Find max for numerical stability.
		var maxVal float32 = x[start]
		for i := 1; i < dim; i++ {
			if x[start+i] > maxVal {
				maxVal = x[start+i]
			}
		}
		var sum float32
		for i := 0; i < dim; i++ {
			x[start+i] = float32(math.Exp(float64(x[start+i] - maxVal)))
			sum += x[start+i]
		}
		for i := 0; i < dim; i++ {
			x[start+i] /= sum
		}
	}
}

// transpose in-place: assumes square matrix or copies to output.
// For attention, we typically need the general case.
func transpose(src []float32, dst []float32, rows, cols int) {
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			dst[j*rows+i] = src[i*cols+j]
		}
	}
}

// copyMatrix copies src to dst.
func copyMatrix(dst, src []float32) {
	copy(dst, src)
}

// scale multiplies all elements by scalar in-place.
func scale(x []float32, s float32) {
	for i := range x {
		x[i] *= s
	}
}

// meanPool computes mean over the sequence dimension [seq_len × dim] → [dim].
func meanPool(hidden []float32, seqLen, dim int) []float32 {
	result := make([]float32, dim)
	for i := 0; i < seqLen; i++ {
		for j := 0; j < dim; j++ {
			result[j] += hidden[i*dim+j]
		}
	}
	scale(result, 1.0/float32(seqLen))
	return result
}

// clsToken extracts the [CLS] token embedding from [seq_len × dim].
func clsToken(hidden []float32, dim int) []float32 {
	result := make([]float32, dim)
	copy(result, hidden[:dim])
	return result
}

// argmax returns the index of the largest value.
func argmax(x []float32) int {
	if len(x) == 0 {
		return -1
	}
	idx := 0
	for i := 1; i < len(x); i++ {
		if x[i] > x[idx] {
			idx = i
		}
	}
	return idx
}