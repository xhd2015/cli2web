# cli2web

Converts your CLI to web interfaces.

# Install
```sh
go install github.com/xhd2015/cli2web@latest
```

# Usage
Run the tool with a JSON schema file:
```bash
# via schema.json
cli2web --schema schema.json

# via stdin
cat schema.json | cli2web

# via cli's self-hosted schema(suppose `cli schema` output its own schema)
cli schema | cli2web
```

# Example `schema.json`

> See [schema-example.json](schema-example.json).

```json
{
    "root": "kool",
    "description": "amend cli utilities",
    "commands": [
        {
            "name": "git",
            "description": "Git commands",
            "commands": [
                {
                    "name": "tag-next",
                    "description": "Get the next git tag",
                    "examples": [
                        {
                            "usage": "kool git tag-next",
                            "description": "Get the next git tag"
                        }
                    ],
                    "options": [
                        {
                            "flags": "--push",
                            "description": "Pre-release version",
                            "type": "boolean"
                        }
                    ],
                    "output": {
                        "type": "text",
                        "description": "The next git tag"
                    }
                }
            ]
        }
    ]
}
```

# Features
- [x] options
- [x] arguments
- [x] auto select port and open
- [x] bool options as checkbox
- [ ] predefined options(dropdown)
- [ ] allow uploading from file
- [ ] allow stdin interaction
- [ ] mark non-leaf command runnable
- [ ] support variadic arguments
- [ ] mark options required
- [x] schema from markjson directory
- [x] markjson examples
- [ ] auto generate schema from cli help using LLM