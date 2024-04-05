package machinepools

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation"

	commonUtils "github.com/openshift-online/ocm-common/pkg/utils"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/interactive"
	"github.com/openshift/rosa/pkg/rosa"
)

// To clear existing labels in interactive mode, the user enters "" as an empty list value
const (
	interactiveModeEmptyLabels = `""`
	nodeDrainUnitMinute        = "minute"
	nodeDrainUnitMinutes       = "minutes"
	nodeDrainUnitHour          = "hour"
	nodeDrainUnitHours         = "hours"
	nodeDrainUnits             = nodeDrainUnitMinute + "|" + nodeDrainUnitMinutes + "|" +
		nodeDrainUnitHour + "|" + nodeDrainUnitHours
	MaxNodeDrainTimeInMinutes = 10080
	MaxNodeDrainTimeInHours   = 168
)

func MinNodePoolReplicaValidator(autoscaling bool) interactive.Validator {
	return func(val interface{}) error {
		minReplicas, err := strconv.Atoi(fmt.Sprintf("%v", val))
		if err != nil {
			return err
		}
		if autoscaling {
			if minReplicas < 1 {
				return fmt.Errorf("min-replicas must be greater than zero")
			}
		} else {
			if minReplicas < 0 {
				return fmt.Errorf("replicas must be a non-negative integer")
			}
		}
		return nil
	}
}

func MaxNodePoolReplicaValidator(minReplicas int) interactive.Validator {
	return func(val interface{}) error {
		maxReplicas, err := strconv.Atoi(fmt.Sprintf("%v", val))
		if err != nil {
			return err
		}
		if minReplicas > maxReplicas {
			return fmt.Errorf("max-replicas must be greater or equal to min-replicas")
		}
		return nil
	}
}

func ParseLabels(labels string) (map[string]string, error) {
	labelMap := make(map[string]string)
	if labels == "" || labels == interactiveModeEmptyLabels {
		return labelMap, nil
	}

	for _, label := range strings.Split(labels, ",") {
		if !strings.Contains(label, "=") {
			return nil, fmt.Errorf("Expected key=value format for labels")
		}
		tokens := strings.Split(label, "=")
		err := ValidateLabelKeyValuePair(tokens[0], tokens[1])
		if err != nil {
			return nil, err
		}
		key := strings.TrimSpace(tokens[0])
		value := strings.TrimSpace(tokens[1])
		if _, exists := labelMap[key]; exists {
			return nil, fmt.Errorf("Duplicated label key '%s' used", key)
		}
		labelMap[key] = value
	}
	return labelMap, nil
}

func GetTaints(cmd *cobra.Command, r *rosa.Runtime, existingTaints []*cmv1.Taint,
	inputTaints string) []*cmv1.TaintBuilder {
	if interactive.Enabled() {
		if inputTaints == "" {
			for _, taint := range existingTaints {
				if taint == nil {
					continue
				}
				if inputTaints != "" {
					inputTaints += ","
				}
				inputTaints += fmt.Sprintf("%s=%s:%s", taint.Key(), taint.Value(), taint.Effect())
			}
		}
		var err error
		inputTaints, err = interactive.GetString(interactive.Input{
			Question: "Taints",
			Help:     cmd.Flags().Lookup("taints").Usage,
			Default:  inputTaints,
			Validators: []interactive.Validator{
				taintValidator,
			},
		})
		if err != nil {
			r.Reporter.Errorf("Expected a valid comma-separated list of attributes: %s", err)
			os.Exit(1)
		}
	}
	taintBuilders, err := ParseTaints(inputTaints)
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}
	return taintBuilders
}

func ParseTaints(taints string) ([]*cmv1.TaintBuilder, error) {
	taintBuilders := []*cmv1.TaintBuilder{}
	if taints == "" || taints == interactiveModeEmptyLabels {
		return taintBuilders, nil
	}
	var errs []error
	for _, taint := range strings.Split(taints, ",") {
		if !strings.Contains(taint, "=") || !strings.Contains(taint, ":") {
			return nil, fmt.Errorf("Expected key=value:scheduleType format for taints. Got '%s'", taint)
		}
		// First split effect
		splitEffect := strings.Split(taint, ":")
		// Then split key and value
		splitKeyValue := strings.Split(splitEffect[0], "=")
		newTaintBuilder := cmv1.NewTaint().Key(splitKeyValue[0]).Value(splitKeyValue[1]).Effect(splitEffect[1])
		newTaint, _ := newTaintBuilder.Build()
		if err := ValidateTaintKeyValuePair(newTaint.Key(), newTaint.Value()); err != nil {
			errs = append(errs, err)
			continue
		}
		if newTaint.Effect() == "" {
			// Note: an empty effect means any effect. For the moment this is not supported
			errs = append(errs, fmt.Errorf("Expected a not empty effect"))
			continue
		}
		taintBuilders = append(taintBuilders, newTaintBuilder)
	}

	if len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}

	return taintBuilders, nil
}

func ValidateTaintKeyValuePair(key, value string) error {
	return ValidateKeyValuePair(key, value, "taint")
}

func ValidateLabelKeyValuePair(key, value string) error {
	return ValidateKeyValuePair(key, value, "label")
}

func ValidateKeyValuePair(key, value string, resourceName string) error {
	if errs := validation.IsQualifiedName(key); len(errs) != 0 {
		return fmt.Errorf("Invalid %s key '%s': %s", resourceName, key, strings.Join(errs, "; "))
	}

	if errs := validation.IsValidLabelValue(value); len(errs) != 0 {
		return fmt.Errorf("Invalid %s value '%s': at key: '%s': %s", resourceName,
			value, key, strings.Join(errs, "; "))
	}
	return nil
}

func taintValidator(val interface{}) error {
	if taints, ok := val.(string); ok {
		_, err := ParseTaints(taints)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("can only validate strings, got %v", val)
}

func GetLabelMap(cmd *cobra.Command, r *rosa.Runtime, existingLabels map[string]string,
	inputLabels string) map[string]string {
	if interactive.Enabled() {
		if inputLabels == "" {
			for lk, lv := range existingLabels {
				if inputLabels != "" {
					inputLabels += ","
				}
				inputLabels += fmt.Sprintf("%s=%s", lk, lv)
			}
		}
		var err error
		inputLabels, err = interactive.GetString(interactive.Input{
			Question: "Labels",
			Help:     cmd.Flags().Lookup("labels").Usage,
			Default:  inputLabels,
			Validators: []interactive.Validator{
				LabelValidator,
			},
		})
		if err != nil {
			r.Reporter.Errorf("Expected a valid comma-separated list of attributes: %s", err)
			os.Exit(1)
		}
	}
	labelMap, err := ParseLabels(inputLabels)
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}
	return labelMap
}

func LabelValidator(val interface{}) error {
	if labels, ok := val.(string); ok {
		_, err := ParseLabels(labels)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("can only validate strings, got %v", val)
}

func HostedClusterOnlyFlag(r *rosa.Runtime, cmd *cobra.Command, flagName string) {
	isFlagSet := cmd.Flags().Changed(flagName)
	if isFlagSet {
		r.Reporter.Errorf("Setting the `%s` flag is only supported for hosted clusters", flagName)
		os.Exit(1)
	}
}

func CreateNodeDrainGracePeriodBuilder(nodeDrainGracePeriod string) (*cmv1.ValueBuilder, error) {
	valueBuilder := cmv1.NewValue()
	if nodeDrainGracePeriod == "" {
		return valueBuilder, nil
	}

	nodeDrainParsed := strings.Split(nodeDrainGracePeriod, " ")
	nodeDrainValue, err := strconv.ParseFloat(nodeDrainParsed[0], commonUtils.MaxByteSize)
	if err != nil {
		return nil, fmt.Errorf("Invalid time for the node drain grace period: %s", err)
	}

	// Default to minutes if no unit is specified
	if len(nodeDrainParsed) > 1 {
		if nodeDrainParsed[1] == nodeDrainUnitHours || nodeDrainParsed[1] == nodeDrainUnitHour {
			nodeDrainValue = nodeDrainValue * 60
		}
	}

	valueBuilder.Value(nodeDrainValue).Unit("minutes")
	return valueBuilder, nil
}

func ValidateNodeDrainGracePeriod(val interface{}) error {
	nodeDrainGracePeriod := val.(string)
	if nodeDrainGracePeriod == "" {
		return nil
	}

	nodeDrainParsed := strings.Split(nodeDrainGracePeriod, " ")
	if len(nodeDrainParsed) > 2 {
		return fmt.Errorf("Expected format to include the duration and "+
			"the unit (%s).", nodeDrainUnits)
	}
	nodeDrainValue, err := strconv.ParseInt(nodeDrainParsed[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid value '%s', the duration must be an integer.",
			nodeDrainParsed[0])
	}

	// Default to minutes if no unit is specified
	if len(nodeDrainParsed) > 1 {
		if nodeDrainParsed[1] != nodeDrainUnitHours && nodeDrainParsed[1] != nodeDrainUnitHour &&
			nodeDrainParsed[1] != "minutes" && nodeDrainParsed[1] != "minute" {
			return fmt.Errorf("Invalid unit '%s', value unit is '%s'", nodeDrainParsed[1], nodeDrainUnits)
		}
		if nodeDrainParsed[1] == nodeDrainUnitHours || nodeDrainParsed[1] == nodeDrainUnitHour {
			if nodeDrainValue > MaxNodeDrainTimeInHours {
				return fmt.Errorf("Value '%v' cannot exceed the maximum of %d hours "+
					"(1 week)", nodeDrainValue, MaxNodeDrainTimeInHours)
			}
			nodeDrainValue = nodeDrainValue * 60
		}
	}
	if nodeDrainValue < 0 {
		return fmt.Errorf("Value '%v' cannot be negative", nodeDrainValue)
	}
	if nodeDrainValue > MaxNodeDrainTimeInMinutes {
		return fmt.Errorf("Value '%v' cannot exceed the maximum of %d minutes "+
			"(1 week)", nodeDrainValue, MaxNodeDrainTimeInMinutes)
	}
	return nil
}
