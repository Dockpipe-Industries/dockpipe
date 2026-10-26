# Candidate publication contract

Publishers select their destination from explicit `--repository owner/name` or the GitHub Actions `GITHUB_REPOSITORY` context. Missing or malformed targets fail before network access. Every GraphQL query, release lookup, create, upload and edit receives that same validated destination; shared transactions contain no deployment target default.

`source-identity.json` pins the accepted artifact origin. It is provenance policy, separate from the destination. Publication still requires an accepted `dev` push from that source repository, its exact commit, qualified artifact metadata and matching immutable tag/digest inventory. An operator may explicitly select another destination for that accepted source; selecting a destination never changes artifact provenance.

CLI version allocation reads the selected destination's published tag history. Its receipt binds that repository URL, source commit, bounded history, digest and allocated version; a publisher cannot reuse it for another destination. SDK source/ABI qualification remains artifact-specific.
