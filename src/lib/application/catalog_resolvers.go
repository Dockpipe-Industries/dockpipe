package application

import "dockpipe/src/lib/infrastructure"

func catalogResolverDetails(workdir string, names []string) []infrastructure.ResolverMetadata {
	out := make([]infrastructure.ResolverMetadata, 0, len(names))
	for _, name := range names {
		profile, err := infrastructure.ResolveResolverFilePath(workdir, name)
		if err != nil {
			continue
		}
		metadata, err := infrastructure.DescribeResolver(profile, name)
		if err == nil {
			out = append(out, metadata)
		}
	}
	return out
}
