//go:build windows

package main

import "golang.org/x/sys/windows"

func detectSystemLocaleCode() string {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err == nil {
		for _, language := range languages {
			if code := normalizeHostLocaleCode(language); code != hostLocaleEN || language == "" {
				return code
			}
		}
	}
	return hostLocaleEN
}
