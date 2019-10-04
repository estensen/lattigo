package main

import (
	"fmt"
	"log"
	"math"
	"math/cmplx"
	"math/rand"
	"sort"
	"time"

	"github.com/ldsec/lattigo/ckks"
)

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func randomComplex(min, max float64) complex128 {
	return complex(randomFloat(min, max), randomFloat(min, max))
}

func chebyshevinterpolation() {

	// This example will pack random 8192 float64 values in the range [-8, 8] and approximate
	// the function 1/(exp(-x) + 1) over the range [-8, 8].
	// The result is then parsed and compared to the expected result.

	rand.Seed(time.Now().UnixNano())

	var err error

	// Scheme params
	params := ckks.DefaultParams[14]

	// Context
	var ckkscontext *ckks.CkksContext
	if ckkscontext, err = ckks.NewCkksContext(params); err != nil {
		log.Fatal(err)
	}

	encoder := ckkscontext.NewEncoder()

	// Keys
	kgen := ckkscontext.NewKeyGenerator()
	var sk *ckks.SecretKey
	var pk *ckks.PublicKey
	sk, pk = kgen.NewKeyPair()

	// Relinearization key
	var rlk *ckks.EvaluationKey
	if rlk, err = kgen.NewRelinKey(sk, ckkscontext.Scale()); err != nil {
		log.Fatal(err)
	}

	// Encryptor
	var encryptor *ckks.Encryptor
	if encryptor, err = ckkscontext.NewEncryptorFromPk(pk); err != nil {
		log.Fatal(err)
	}

	// Decryptor
	var decryptor *ckks.Decryptor
	if decryptor, err = ckkscontext.NewDecryptor(sk); err != nil {
		log.Fatal(err)
	}

	// Evaluator
	var evaluator *ckks.Evaluator
	if evaluator = ckkscontext.NewEvaluator(); err != nil {
		log.Fatal(err)
	}

	// Values to encrypt
	values := make([]complex128, ckkscontext.Slots())
	for i := range values {
		values[i] = complex(randomFloat(-8, 8), 0)
	}

	fmt.Printf("HEAAN parameters : logN = %d, logQ = %d, levels = %d, logScale = %d, sigma = %f \n",
		ckkscontext.LogN(), ckkscontext.LogQ(), ckkscontext.Levels(), ckkscontext.Scale(), ckkscontext.Sigma())

	fmt.Println()
	fmt.Printf("Values     : %6f %6f %6f %6f...\n",
		round(values[0]), round(values[1]), round(values[2]), round(values[3]))
	fmt.Println()

	// Plaintext creation and encoding process
	plaintext := ckkscontext.NewPlaintext(ckkscontext.Levels()-1, ckkscontext.Scale())
	if err = encoder.EncodeComplex(plaintext, values); err != nil {
		log.Fatal(err)
	}

	// Encryption process
	var ciphertext *ckks.Ciphertext
	if ciphertext, err = encryptor.EncryptNew(plaintext); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Evaluation of the function 1/(exp(-x)+1) in the range [-8, 8] (degree of approximation : 32)")

	// Evaluation process
	chebyapproximation := ckks.Approximate(f, -8, 8, 33) // We ask to approximate f(x) in
	// the range [-8, 8] with a chebyshev polynomial of 33 coefficients (degree 32).

	if ciphertext, err = evaluator.EvaluateCheby(ciphertext, chebyapproximation, rlk); err != nil {
		// We evaluate the interpolated chebyshev polynomial on the ciphertext
		log.Fatal(err)
	}

	fmt.Println("Done... Consumed levels :", ckkscontext.Levels()-1-ciphertext.Level())

	// Decryption process + Decoding process
	valuesTest := encoder.DecodeComplex(decryptor.DecryptNew(ciphertext))

	// Computation of the reference values
	for i := range values {
		values[i] = f(values[i])
	}

	// Printing results and comparison
	fmt.Println()
	fmt.Printf("ValuesTest : %6f %6f %6f %6f...\n",
		round(valuesTest[0]), round(valuesTest[1]), round(valuesTest[2]), round(valuesTest[3]))
	fmt.Printf("ValuesWant : %6f %6f %6f %6f...\n",
		round(values[0]), round(values[1]), round(values[2]), round(values[3]))
	verify_vector(values, valuesTest)

}

func f(x complex128) complex128 {
	return 1 / (cmplx.Exp(-x) + 1)
}

func round(x complex128) complex128 {
	var factor float64
	factor = 100000000
	a := math.Round(real(x)*factor) / factor
	b := math.Round(imag(x)*factor) / factor
	return complex(a, b)
}

func verify_vector(valuesWant, valuesTest []complex128) (err error) {

	var deltaReal, deltaImag float64

	var minprec, maxprec, meanprec, medianprec complex128

	diff := make([]complex128, len(valuesWant))

	minprec = complex(0, 0)
	maxprec = complex(1, 1)

	meanprec = complex(0, 0)

	distrib_real := make(map[uint64]uint64)
	distrib_imag := make(map[uint64]uint64)

	for i := range valuesWant {

		// Test the ratio for big values (> 1) and difference for small values (< 1)

		deltaReal = math.Abs(real(valuesTest[i]) - real(valuesWant[i]))
		deltaImag = math.Abs(imag(valuesTest[i]) - imag(valuesWant[i]))

		diff[i] += complex(deltaReal, 0)
		diff[i] += complex(0, deltaImag)

		meanprec += diff[i]

		if real(diff[i]) > real(minprec) {
			minprec = complex(real(diff[i]), 0)
		}

		if imag(diff[i]) > imag(minprec) {
			minprec = complex(real(minprec), imag(diff[i]))
		}

		if real(diff[i]) < real(maxprec) {
			maxprec = complex(real(diff[i]), 0)
		}

		if imag(diff[i]) < imag(maxprec) {
			maxprec = complex(real(maxprec), imag(diff[i]))
		}

		distrib_real[uint64(math.Floor(math.Log2(1/real(diff[i]))))] += 1
		distrib_imag[uint64(math.Floor(math.Log2(1/imag(diff[i]))))] += 1
	}

	meanprec /= complex(float64(len(valuesWant)), 0)
	medianprec = calcmedian(diff)

	fmt.Println()
	fmt.Printf("Minimum precision : (%.2f, %.2f) bits \n",
		math.Log2(1/real(minprec)), math.Log2(1/imag(minprec)))
	fmt.Printf("Maximum precision : (%.2f, %.2f) bits \n",
		math.Log2(1/real(maxprec)), math.Log2(1/imag(maxprec)))
	fmt.Printf("Mean    precision : (%.2f, %.2f) bits \n",
		math.Log2(1/real(meanprec)), math.Log2(1/imag(meanprec)))
	fmt.Printf("Median  precision : (%.2f, %.2f) bits \n",
		math.Log2(1/real(medianprec)), math.Log2(1/imag(medianprec)))
	fmt.Println()

	return nil
}

func calcmedian(values []complex128) (median complex128) {

	tmp := make([]float64, len(values))

	for i := range values {
		tmp[i] = real(values[i])
	}

	sort.Float64s(tmp)

	for i := range values {
		values[i] = complex(tmp[i], imag(values[i]))
	}

	for i := range values {
		tmp[i] = imag(values[i])
	}

	sort.Float64s(tmp)

	for i := range values {
		values[i] = complex(real(values[i]), tmp[i])
	}

	index := len(values) / 2

	if len(values)&1 == 1 {
		return values[index]
	}

	return (values[index] + values[index+1]) / 2

}

func main() {
	chebyshevinterpolation()
}
