package application

import (
	"maps"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// runSession owns the mutable workflow environment. Initial command-only values
// remain an explicit overlay until setup exports replace them, as in the CLI contract.
// Container environment projection is deliberately owned separately.
type runSession struct {
	env             map[string]string
	launchOverrides map[string]string
	services        runExecutionServices
}

func newRunSession(environment []string, services runExecutionServices) *runSession {
	return &runSession{
		env:      domain.EnvironToMap(environment),
		services: services,
	}
}

func (session *runSession) commandEnvironment() []string {
	if len(session.launchOverrides) == 0 {
		return domain.EnvMapToSlice(session.env)
	}
	environment := maps.Clone(session.env)
	maps.Copy(environment, session.launchOverrides)
	return domain.EnvMapToSlice(environment)
}

func (session *runSession) mergeSetupEnvironment(environment map[string]string) {
	maps.Copy(session.env, environment)
	session.launchOverrides = nil
}

// runCompletion describes success-only finalization. It is not a cleanup defer:
// finalizing a workspace can checkpoint changes and must never run on failure.
type runCompletion struct {
	workflow       *domain.Workflow
	gitSession     *infrastructure.GitSession
	repoRoot       string
	workdir        string
	bundleOut      string
	afterScripts   []string
	extraEnv       []string
	strategyCommit bool
	profile        string
	template       string
}

func (session *runSession) complete(completion runCompletion, commitWorkdir string, commitOnHost bool) error {
	if completion.strategyCommit {
		mergeCommitEnvFromLines(session.env, completion.extraEnv)
		applyBranchPrefix(session.env, completion.profile, completion.template)
		session.launchOverrides = nil
	}
	if len(completion.afterScripts) > 0 {
		if err := runStrategyAfterScripts(
			completion.afterScripts, completion.repoRoot, session.env, session.commandEnvironment(),
			completion.workdir, completion.bundleOut, session.services.hostScript, session.services.commit,
		); err != nil {
			return err
		}
	}
	if err := session.services.finalize(completion.gitSession, completion.workflow); err != nil {
		return err
	}
	if commitOnHost && !completion.strategyCommit {
		return session.services.commit(
			commitWorkdir, session.env["DOCKPIPE_COMMIT_MESSAGE"],
			firstNonEmpty(session.env["DOCKPIPE_BUNDLE_OUT"], completion.bundleOut),
			strings.TrimSpace(session.env["DOCKPIPE_BUNDLE_ALL"]) == "1",
		)
	}
	return nil
}
