package main

import (
	"math/big"
	"testing"
)

// ─── hexToBigInt Tests ────────────────────────────────────────────────────────

func TestHexToBigInt_ValidHex(t *testing.T) {
	result, err := hexToBigInt("0x1a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Int64() != 26 {
		t.Errorf("expected 26, got %d", result.Int64())
	}
}

func TestHexToBigInt_WithoutPrefix(t *testing.T) {
	result, err := hexToBigInt("ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Int64() != 255 {
		t.Errorf("expected 255, got %d", result.Int64())
	}
}

func TestHexToBigInt_Zero(t *testing.T) {
	result, err := hexToBigInt("0x0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Int64() != 0 {
		t.Errorf("expected 0, got %d", result.Int64())
	}
}

func TestHexToBigInt_InvalidHex(t *testing.T) {
	_, err := hexToBigInt("0xZZZZ")
	if err == nil {
		t.Error("expected error for invalid hex, got nil")
	}
}

func TestHexToBigInt_LargeValue(t *testing.T) {
	// 1 ETH in Wei = 1000000000000000000 = 0xDE0B6B3A7640000
	result, err := hexToBigInt("0xDE0B6B3A7640000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := new(big.Int)
	expected.SetString("1000000000000000000", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("expected %s, got %s", expected.String(), result.String())
	}
}

// ─── weiToEth Tests ───────────────────────────────────────────────────────────

func TestWeiToEth_OneEth(t *testing.T) {
	oneEth := new(big.Int)
	oneEth.SetString("1000000000000000000", 10) // 1e18
	result := weiToEth(oneEth)
	if result != "1.000000" {
		t.Errorf("expected 1.000000, got %s", result)
	}
}

func TestWeiToEth_Zero(t *testing.T) {
	result := weiToEth(big.NewInt(0))
	if result != "0.000000" {
		t.Errorf("expected 0.000000, got %s", result)
	}
}

func TestWeiToEth_Nil(t *testing.T) {
	result := weiToEth(nil)
	if result != "0.000000" {
		t.Errorf("expected 0.000000 for nil, got %s", result)
	}
}

func TestWeiToEth_HalfEth(t *testing.T) {
	halfEth := new(big.Int)
	halfEth.SetString("500000000000000000", 10) // 0.5 ETH
	result := weiToEth(halfEth)
	if result != "0.500000" {
		t.Errorf("expected 0.500000, got %s", result)
	}
}
