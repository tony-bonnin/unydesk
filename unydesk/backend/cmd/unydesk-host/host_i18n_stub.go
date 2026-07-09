//go:build !windows

package main

func detectSystemLocaleCode() string {
	return hostLocaleEN
}
