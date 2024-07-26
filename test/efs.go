package test

import "embed"

//go:embed testdata/*
var TestData embed.FS
