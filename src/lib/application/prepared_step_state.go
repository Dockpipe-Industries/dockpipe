package application

import "dockpipe/src/lib/domain"

// preparedStepState owns one step's effective environment projection and output
// location. Blocking steps deliberately share the sequential maps; async callers
// provide private clones. Container environment remains separate from host state.
type preparedStepState struct {
	env         map[string]string
	dockerEnv   map[string]string
	outputsPath string
}

func prepareStepState(options *runStepsOpts, step domain.Step, index int, env, dockerEnv map[string]string) (preparedStepState, error) {
	state := preparedStepState{env: env, dockerEnv: dockerEnv}
	if err := applyStepEnvOverrides(options, step, index, state.env, state.dockerEnv); err != nil {
		return preparedStepState{}, err
	}
	state.refreshOutputsPath(options, step)
	return state, nil
}

func (state *preparedStepState) refreshOutputsPath(options *runStepsOpts, step domain.Step) {
	// Sourced setup can alter workdir or scope after initial path admission.
	state.outputsPath = stepOutputsAbsPath(options, step, state.env)
}

func (state *preparedStepState) hostEnvironment() []string {
	return domain.EnvMapToSlice(state.env)
}
