// +build ignore

package main

import (
	"strings"

	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

func main() {
	generate("and")
	generate("andnot")
	generate("or")
	generate("xor")
	Generate()
}

// generate generates an SIMD "and", "or" , "andnot"operations.
func generate(op string) {
	switch op {
	case "and":
		TEXT("And", NOSPLIT, "func(a []uint64, b []uint64)")
		Doc("And And computes the intersection between two slices and stores the result in the first one")
	case "or":
		TEXT("Or", NOSPLIT, "func(a []uint64, b []uint64)")
		Doc("Or computes the union between two slices and stores the result in the first one")
	case "andnot":
		TEXT("AndNot", NOSPLIT, "func(a []uint64, b []uint64)")
		Doc("AndNot computes the difference between two slices and stores the result in the first one")
	case "xor":
		TEXT("Xor", NOSPLIT, "func(a []uint64, b []uint64)")
		Doc("Xor computes the symmetric difference between two slices and stores the result in the first one")
	}

	// Load the a and b addresses as well as the current len(a). Assume len(a) == len(b)
	a := Mem{Base: Load(Param("a").Base(), GP64())}
	b := Mem{Base: Load(Param("b").Base(), GP64())}
	n := Load(Param("a").Len(), GP64())

	// The register for the tail, we xor it with itself to zero out
	s := GP64()
	XORQ(s, s)

	const size, unroll = 32, 2 // bytes (256bit * 2)
	const blocksize = size * unroll

	Commentf("perform vectorized operation for every block of %v bits", blocksize*8)
	Label("body")
	CMPQ(n, U32(4*unroll))
	JL(LabelRef("tail"))

	// Create a vector
	vector := make([]VecVirtual, unroll)
	for i := 0; i < unroll; i++ {
		vector[i] = YMM()
	}

	// Move memory vector into position
	Commentf("perform the logical \"%v\" operation", strings.ToUpper(op))
	for i := 0; i < unroll; i++ {
		VMOVUPD(b.Offset(size*i), vector[i])
	}

	// Perform the actual operation
	for i := 0; i < unroll; i++ {
		switch op {
		case "and":
			VPAND(a.Offset(size*i), vector[i], vector[i])
		case "or":
			VPOR(a.Offset(size*i), vector[i], vector[i])
		case "andnot":
			VPANDN(a.Offset(size*i), vector[i], vector[i])
		case "xor":
			VPXOR(a.Offset(size*i), vector[i], vector[i])
		}
	}

	// Move the result to "a" by copying the vector
	for i := 0; i < unroll; i++ {
		VMOVUPD(vector[i], a.Offset(size*i))
	}

	// Continue the iteration
	Comment("continue the interation by moving read pointers")
	ADDQ(U32(blocksize), a.Base)
	ADDQ(U32(blocksize), b.Base)
	SUBQ(U32(4*unroll), n)
	JMP(LabelRef("body"))

	// Now, we only have less than 512 bits left, use normal scalar operation
	Label("tail")
	CMPQ(n, Imm(0))
	JE(LabelRef("done"))

	// Perform the actual operation
	Commentf("perform the logical \"%v\" operation", strings.ToUpper(op))
	MOVQ(Mem{Base: b.Base}, s)
	switch op {
	case "and":
		ANDQ(Mem{Base: a.Base}, s)
	case "or":
		ORQ(Mem{Base: a.Base}, s)
	case "andnot":
		ANDNQ(Mem{Base: a.Base}, s, s)
	case "xor":
		XORQ(Mem{Base: a.Base}, s)
	}
	MOVQ(s, Mem{Base: a.Base})

	// Continue the iteration
	Comment("continue the interation by moving read pointers")
	ADDQ(U32(8), a.Base)
	ADDQ(U32(8), b.Base)
	SUBQ(U32(1), n)
	JMP(LabelRef("tail"))

	Label("done")
	RET()
}
