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

type OneAgentMetadataEnricher struct {
	logger *zap.Logger
}

func NewOneAgentMetadataEnricher(log *zap.Logger) *OneAgentMetadataEnricher {
	return &OneAgentMetadataEnricher{logger: log}
}

func readIndirectionFile(reader io.Reader, prefix string) (string, error) {
	if reader == nil {
		return "", errors.New("reader cannot be nil")
	}

	scanner := bufio.NewScanner(reader)
	indirectionFilename := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			indirectionFilename = line
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return indirectionFilename, nil
}

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

func readOneAgentMetadata(indirectionBasename string) ([]string, error) {
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

type KeyValuePair struct {
	Key   string
	Value string
}

func newKeyValuePair(key, value string) KeyValuePair {
	return KeyValuePair{
		Key:   key,
		Value: value,
	}
}

func (o OneAgentMetadataEnricher) parseOneAgentMetadata(lines []string) []mint.Dimension {
	result := []mint.Dimension{}
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

		result = append(result, mint.NewDimension(key, value))
	}
	return result
}

// append OneAgent metadata tags to the passed slice.
func (o OneAgentMetadataEnricher) EnrichWithMetadata(target []mint.Dimension) {
	lines, err := readOneAgentMetadata(indirectionBasename)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("could not read OneAgent Metadata: %s", err))
		return
	}
	for _, dim := range o.parseOneAgentMetadata(lines) {
		target = append(target, dim)
	}
}
