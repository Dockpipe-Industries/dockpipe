package packagecatalog

// Discovery returns a catalog view without altering the full installation catalog.
// Hidden entries remain available for explicit installs and dependency resolution.
func (c Catalog) Discovery(includeHidden bool) Catalog {
	visible := c
	visible.Packages = make([]Entry, 0, len(c.Packages))
	for _, entry := range c.Packages {
		if includeHidden || !entry.Hidden {
			visible.Packages = append(visible.Packages, entry)
		}
	}
	return visible
}
