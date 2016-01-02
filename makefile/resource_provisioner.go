package makefile

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/armon/circbuf"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-homedir"
	"github.com/mitchellh/go-linereader"
	"github.com/mitchellh/mapstructure"
)

const (
	// maxBufSize limits how much output we collect from a local
	// invocation. This is to prevent TF memory usage from growing
	// to an enormous amount due to a faulty process.
	maxBufSize = 8 * 1024
)

// Makefile represents a Makefile configuration
type Makefile struct {
	Directory string      `mapstructure:"directory"`
	Target    string      `mapstructure:"target"`
	Variables interface{} `mapstructure:"variables"`
}

// ResourceProvisioner represents a generic Makefile provisioner
type ResourceProvisioner struct{}

// Apply executes make
func (p *ResourceProvisioner) Apply(
	o terraform.UIOutput,
	s *terraform.InstanceState,
	c *terraform.ResourceConfig) error {

	m, err := p.decodeConfig(c)
	if err != nil {
		return err
	}

	// Execute make via a shell
	pr, pw := io.Pipe()
	copyDoneCh := make(chan struct{})
	go p.copyOutput(o, pr, copyDoneCh)

	// Format the variables into k=v pairs
	var makeArgs []string
	makeArgs = append(makeArgs, m.Target)
	for k, v := range m.Variables.(map[string]interface{}) {
		makeArgs = append(makeArgs, fmt.Sprintf("%s=%s", k, v.(string)))
	}

	// run the command
	cmd := exec.Command("make", makeArgs...)
	cmd.Dir = m.Directory
	output, _ := circbuf.NewBuffer(maxBufSize)
	cmd.Stderr = io.MultiWriter(output, pw)
	cmd.Stdout = io.MultiWriter(output, pw)

	// Build the full command for output
	fullCommand := fmt.Sprintf("make %s %s", m.Target, strings.Join(makeArgs, " "))

	// Output what we're about to run
	o.Output(fmt.Sprintf("Executing: %s", fullCommand))

	// Run the command to completion
	err = cmd.Run()

	// Close the write-end of the pipe so that the goroutine mirroring output
	// ends properly.
	pw.Close()
	<-copyDoneCh

	if err != nil {
		return fmt.Errorf("Error running command '%s': %v. Output: %s", fullCommand, err, output.Bytes())
	}

	return nil

}

// Validate checks if the required arguments are configured
func (p *ResourceProvisioner) Validate(c *terraform.ResourceConfig) (ws []string, es []error) {
	m, err := p.decodeConfig(c)
	if err != nil {
		es = append(es, err)
		return ws, es
	}

	if m.Directory == "" {
		es = append(es, fmt.Errorf("A directory is required."))
	}

	if m.Target == "" {
		es = append(es, fmt.Errorf("A target is required."))
	}

	return ws, es
}

func (p *ResourceProvisioner) decodeConfig(c *terraform.ResourceConfig) (*Makefile, error) {
	makeFile := new(Makefile)

	decConf := &mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           makeFile,
	}
	dec, err := mapstructure.NewDecoder(decConf)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	for k, v := range c.Raw {
		m[k] = v
	}

	for k, v := range c.Config {
		m[k] = v
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	// Collect variables
	if vars, ok := c.Config["variables"]; ok {
		makeFile.Variables, err = rawToJSON(vars)
		if err != nil {
			return nil, fmt.Errorf("Error parsing the variables: %v", err)
		}
	}

	// Expand home on possible areas
	makeFile.Directory, err = homedir.Expand(makeFile.Directory)
	if err != nil {
		return nil, err
	}

	return makeFile, nil
}

func rawToJSON(raw interface{}) (interface{}, error) {
	switch s := raw.(type) {
	case []map[string]interface{}:
		if len(s) != 1 {
			return nil, fmt.Errorf("unexpected input while parsing raw config to JSON")
		}

		var err error
		for k, v := range s[0] {
			s[0][k], err = rawToJSON(v)
			if err != nil {
				return nil, err
			}
		}

		return s[0], nil
	default:
		return raw, nil
	}
}

func (p *ResourceProvisioner) copyOutput(
	o terraform.UIOutput, r io.Reader, doneCh chan<- struct{}) {
	defer close(doneCh)
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}
