//go:build !fips

package main

func checkCryptoMode() error { return nil }

func cryptoBuildInfo() string { return "" }
