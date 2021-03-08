// Copyright 2021 Dynatrace LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package oneagentenrichment

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dynatrace-oss/opentelemetry-metric-go/mint"
	"go.uber.org/zap"
)

const indirectionBasename = "dt_metadata_e617c525669e072eebe3d0f08212e8f2"

// OneAgentMetadataEnricher contains the functionality to read and parse
// OneAgent metadata into key-value pairs.
type OneAgentMetadataEnricher struct {
	logger *zap.Logger
}

// NewOneAgentMetadataEnricher creates a new OneAgentMetadataEnricher with an attached logger.
func NewOneAgentMetadataEnricher(log *zap.Logger) OneAgentMetadataEnricher {
	return OneAgentMetadataEnricher{
		logger: log,
	}
}

// readIndirectionFile reads the text from the Reader line by line and
// returns the first line that contains the prefix
func readIndirectionFile(reader io.Reader, prefix string) (string, error) {
	if reader == nil {
		return "", errors.New("reader cannot be nil")
	}

	scanner := bufio.NewScanner(reader)
	indirectionFilename := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, prefix) {
			indirectionFilename = line
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return indirectionFilename, nil
}

// readMetadataFile reads the actual OneAgent metadata file,
// trimming whitespace from lines and discarding empty lines.
func readMetadataFile(reader io.Reader) ([]string, error) {
	if reader == nil {
		return nil, errors.New("reader cannot be nil")
	}

	scanner := bufio.NewScanner(reader)
	lines := []string{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// readOneAgentMetadata takes the basename of the properties file (without the .properties)
// suffix. It then reads the indirection file to extract a filename starting with the basename.
// that file is then read and parsed into an array of strings, which represent the lines of
// that file. Errors from function calls inside this function are passed on to the caller.
func readOneAgentMetadata(indirectionBasename string) ([]string, error) {
	// Currently, this only works on Windows hosts, since the indirection on Linux
	// is based on libc. As Go does not use libc to open files, this doesnt currently
	// work on Linux hosts.
	indirection, err := os.Open(fmt.Sprintf("%s.properties", indirectionBasename))
	if err != nil {
		// an error occurred during opening of the file
		return nil, err
	}
	defer indirection.Close()

	filename, err := readIndirectionFile(indirection, indirectionBasename)
	if err != nil {
		// an error occurred during reading of the file
		return nil, err
	}

	if filename == "" {
		return nil, errors.New("metadata file name is empty")
	}

	metadataFile, err := os.Open(filename)
	if err != nil {
		// an error occurred during opening of the file
		return nil, err
	}

	content, err := readMetadataFile(metadataFile)

	if err != nil {
		// an error occurred during reading of the file
		return nil, err
	}

	return content, nil
}

// parseOneAgentMetadata transforms lines into key-value pairs and discards
// pairs that do not conform to the 'key=value' (trailing additional equal signs are added to
// the value)
func (o OneAgentMetadataEnricher) parseOneAgentMetadata(lines []string) []mint.Tag {
	result := []mint.Tag{}
	for _, line := range lines {
		split := strings.SplitN(line, "=", 2)
		if len(split) != 2 {
			o.logger.Warn(fmt.Sprintf("Could not parse OneAgent metadata line '%s'", line))
			continue
		}
		key, value := split[0], split[1]

		if key == "" || value == "" {
			o.logger.Warn(fmt.Sprintf("Could not parse OneAgent metadata line '%s'", line))
			continue
		}

		result = append(result, mint.NewTag(key, value))
	}
	return result
}

// GetOneAgentMetadata reads the metadata and returns it as a slice of Tags.
// No normalisation is done apart from trimming whitespace
func (o OneAgentMetadataEnricher) GetOneAgentMetadata() []mint.Tag {
	tags := []mint.Tag{}
	lines, err := readOneAgentMetadata(indirectionBasename)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("Could not read OneAgent metadata: %s", err))
		return tags
	}

	for _, dim := range o.parseOneAgentMetadata(lines) {
		tags = append(tags, dim)
	}
	return tags
}
