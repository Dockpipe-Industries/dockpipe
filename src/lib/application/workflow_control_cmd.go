package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dockpipe/src/lib/infrastructure"
)

func cmdWorkflowBuildControl(arguments []string) error {
	options := infrastructure.WorkflowControlBuildOptions{}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--help" || argument == "-h" {
			fmt.Println("dockpipe workflow build-control <config.yml> --sdk <absolute-prefix> --sdk-sha256 <digest> [--workdir <path>] [--cxx <path>] [--cmake <path>] [--cxx-profile <profile>] [--sanitizers]")
			return nil
		}
		if argument == "--sanitizers" {
			options.Sanitizers = true
			continue
		}
		if argument != "" && argument[0] != '-' {
			if options.ConfigPath != "" {
				return fmt.Errorf("build-control accepts one workflow file")
			}
			options.ConfigPath = argument
			continue
		}
		if index+1 >= len(arguments) {
			return fmt.Errorf("%s requires a value", argument)
		}
		index++
		value := arguments[index]
		switch argument {
		case "--sdk":
			options.SDKPrefix = value
		case "--sdk-sha256":
			options.SDKSHA256 = value
		case "--workdir":
			options.Workdir = value
		case "--cxx":
			options.CXX = value
		case "--cmake":
			options.CMake = value
		case "--cxx-profile":
			options.ResourceProfile = value
		default:
			return fmt.Errorf("unknown build-control option %s", argument)
		}
	}
	if options.ConfigPath == "" {
		return fmt.Errorf("build-control requires a workflow file")
	}
	absolute, err := filepath.Abs(options.ConfigPath)
	if err != nil {
		return err
	}
	options.ConfigPath = absolute
	if options.Workdir == "" {
		options.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	options.Workdir, err = filepath.Abs(options.Workdir)
	if err != nil {
		return err
	}
	options.Workflow, err = loadWorkflowAppFn(absolute)
	if err != nil {
		return err
	}
	binary, err := infrastructure.BuildWorkflowControl(context.Background(), options)
	if err != nil {
		return err
	}
	fmt.Println(binary)
	return nil
}
