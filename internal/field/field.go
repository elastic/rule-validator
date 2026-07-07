// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package field

type Type string

const (
	TypeAggregateMetricDouble Type = "aggregate_metric_double"
	TypeAlias                 Type = "alias"
	TypeHistogram             Type = "histogram"
	TypeConstantKeyword       Type = "constant_keyword"
	TypeText                  Type = "text"
	TypeMatchOnlyText         Type = "match_only_text"
	TypeKeyword               Type = "keyword"
	TypeLong                  Type = "long"
	TypeInteger               Type = "integer"
	TypeShort                 Type = "short"
	TypeByte                  Type = "byte"
	TypeDouble                Type = "double"
	TypeFloat                 Type = "float"
	TypeHalfFloat             Type = "half_float"
	TypeScaledFloat           Type = "scaled_float"
	TypeDate                  Type = "date"
	TypeDateNanos             Type = "date_nanos"
	TypeBoolean               Type = "boolean"
	TypeBinary                Type = "binary"
	TypeIntegerRange          Type = "integer_range"
	TypeFloatRange            Type = "float_range"
	TypeLongRange             Type = "long_range"
	TypeDoubleRange           Type = "double_range"
	TypeDateRange             Type = "date_range"
	TypeIPRange               Type = "ip_range"
	TypeGroup                 Type = "group"
	TypeGeoPoint              Type = "geo_point"
	TypeObject                Type = "object"
	TypeIP                    Type = "ip"
	TypeNested                Type = "nested"
	TypeFlattened             Type = "flattened"
	TypeWildcard              Type = "wildcard"
	TypeVersion               Type = "version"
	TypeUnsignedLong          Type = "unsigned_long"
	TypeCountedKeyword        Type = "counted_keyword"
	TypeSemanticText          Type = "semantic_text"
	TypeGeoShape              Type = "geo_shape"
)

type Field struct {
	Name   string  `json:"name"`
	Type   Type    `json:"type"`
	Target *string `json:"target,omitempty"` // Target field name, if an alias.
}
