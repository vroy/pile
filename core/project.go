package core

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/chrisdail/pile/gitver"
	"github.com/chrisdail/pile/registry"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
)

const pileConfigName = "pile.yml"
const dockerfile = "Dockerfile"

// ProjectConfig configuration for a project
type ProjectConfig struct {
	// Alternative name for this image. If none specified, defaults to the directory of the project
	Name string

	// Alternate context directory (Directory to "build" the image from)
	ContextDir string `yaml:"context_dir"`

	// Prefix for the container image name
	ImagePrefix string `yaml:"image_prefix"`

	// Prefix to add in front of the calculated version. Useful for SemVer/CalVer or for variations of an image in the same registry
	VersionPrefix string `yaml:"version_prefix"`

	// Template for computing the version strong
	VersionTemplate string `yaml:"version_template"`

	// Relative paths to other projects that this project depends on. These are incorporated into the version string
	DependsOn []string `yaml:"depends_on"`

	// Arguments passed to the build command via `--build-arg`
	BuildArgs map[string]string `yaml:"build_args"`

	// Optional testing
	Test struct {
		// Alternate target in a multi-stage build to use for tests. Build is only successful if the tests succeed
		Target string

		// Copies test results from the container to the local filesystem (via docker cp)
		CopyResults struct {
			// Location to copy files from in the container. Example: /app/build/.
			SrcPath string `yaml:"src_path"`
			// Location to copy files to relative to the project directory. Example: build
			DstPath string `yaml:"dst_path"`
		} `yaml:"copy_results"`
	}

	// Docker registry settings for pushing images to and caching already built images
	Registry registry.Config
}

// Project data about an active project
type Project struct {
	Dir string

	Config            ProjectConfig
	CanBuild          bool
	GitVersion        *gitver.GitVersion
	Repository        string
	Tag               string
	Image             string
	ImageWithRegistry string
}

// Load loads a project given a set of defaults from the root
func (project *Project) Load(defaults *ProjectConfig) error {
	configPath := filepath.Join(project.Dir, pileConfigName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Config file does not exist: %s", configPath)
		return nil
	}
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file %s: %s\n", configPath, err)
		return nil
	}

	err = yaml.Unmarshal(configFile, &project.Config)
	if err != nil {
		log.Printf("Error parsing YAML: %s\n", err)
		return nil
	}

	// Default the name to the directory if not present
	if project.Config.Name == "" {
		project.Config.Name = filepath.Base(project.Dir)
	}

	// Merge in defaults
	mergo.Merge(&project.Config, defaults)

	// Compute build related information for this project

	// If there is no Dockerfile, skip all build related computations
	dockerfilePath := filepath.Join(project.Dir, dockerfile)
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		// No dockerfile
		project.CanBuild = false
		return nil
	} else {
		project.CanBuild = true
	}

	// Load version for this project
	project.GitVersion, err = gitver.New(project.versionedPaths())
	if err != nil {
		return err
	}

	// Compute the version tag for building
	project.Tag, err = project.GitVersion.FormatTemplate(project.Config.VersionTemplate)
	if err != nil {
		return err
	}
	if project.Config.VersionPrefix != "" {
		project.Tag = project.Config.VersionPrefix + project.Tag
	}

	// Compute the image name for this project
	project.Repository = fmt.Sprintf("%s%s", project.Config.ImagePrefix, project.Config.Name)
	project.Image = fmt.Sprintf("%s:%s", project.Repository, project.Tag)
	project.ImageWithRegistry = fmt.Sprintf("%s%s", project.Config.Registry.RegistryPrefix(), project.Image)
	return nil
}

// Computes all directories factored into a version check. This includes the project directory and all dependencies
func (project *Project) versionedPaths() []string {
	paths := []string{project.Dir}

	contextDir := project.ContextDir()
	if project.Dir != contextDir {
		paths = append(paths, contextDir)
	}

	for _, dependency := range project.Config.DependsOn {
		paths = append(paths, filepath.Join(project.Dir, dependency))
	}
	return paths
}

// ContextDir returns the context directory absolute path
func (project *Project) ContextDir() string {
	if project.Config.ContextDir != "" {
		return filepath.Join(project.Dir, project.Config.ContextDir)
	}
	return project.Dir
}
