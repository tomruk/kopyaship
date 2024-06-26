package utils

import "github.com/fatih/color"

var (
	Bold    = color.New(color.Bold)
	Red     = color.New(color.FgRed)
	HiGreen = color.New(color.FgHiGreen)
	BgBlue  = color.New(color.BgBlue, color.FgBlack)
	BgWhite = color.New(color.BgWhite, color.FgBlack)
	Success = color.New(color.FgHiGreen)
	Warn    = color.New(color.FgYellow)
	Error   = color.New(color.FgRed)
)
