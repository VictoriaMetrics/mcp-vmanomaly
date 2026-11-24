package tools

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
)

// schemaReflector keeps the struct-tag schema generation used before mcp-go v1.
// mcp-go v1 reflects schemas with google/jsonschema-go, which only reads a free-text
// description and enum= from the jsonschema tag. The argument structs here also rely on
// required, minimum, maximum, exclusiveMinimum, minItems, maxItems and format.
var schemaReflector = jsonschema.Reflector{
	DoNotReference:            true, // inline everything instead of using $defs
	Anonymous:                 true, // omit auto-generated schema IDs
	AllowAdditionalProperties: true, // omit additionalProperties: false
}

func reflectSchema[T any]() json.RawMessage {
	var zero T
	schema := schemaReflector.Reflect(zero)
	schema.Version = "" // MCP schemas carry no $schema field
	raw, err := json.Marshal(schema)
	if err != nil {
		// Tool schemas are static types; a failure is a programming error caught by tests.
		panic(fmt.Sprintf("cannot marshal JSON schema for %T: %v", zero, err))
	}
	return raw
}

// inputSchema sets a tool input schema generated from the jsonschema tags of T.
func inputSchema[T any]() mcp.ToolOption {
	return mcp.WithRawInputSchema(reflectSchema[T]())
}

// outputSchema sets a tool output schema generated from the jsonschema tags of T.
func outputSchema[T any]() mcp.ToolOption {
	raw := reflectSchema[T]()
	return func(t *mcp.Tool) {
		var schema mcp.ToolOutputSchema
		if err := json.Unmarshal(raw, &schema); err != nil {
			panic(fmt.Sprintf("cannot decode output schema for %s: %v", t.Name, err))
		}
		// The MCP spec requires an object output schema.
		schema.Type = "object"
		t.OutputSchema = schema
	}
}
