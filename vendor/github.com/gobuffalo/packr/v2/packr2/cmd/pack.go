package cmd

func parseArgs(args []string) []string {
	var cargs []string
	for _, a := range args {
		if a == "--legacy" {
			globalOptions.Legacy = true
			continue
		}
		if a == "--verbose" {
			globalOptions.Verbose = true
			continue
		}
		if a == "--silent" {
			globalOptions.Silent = true
			continue
		}
		if a == "--ignore-imports" {
			globalOptions.IgnoreImports = true
			continue
		}
		cargs = append(cargs, a)
	}
	return cargs
}
