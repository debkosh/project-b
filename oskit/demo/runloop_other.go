//go:build !darwin

package main

func runMain(demo func()) { demo() }
