# options
```json
[
    {
        "flags": "--dry-run",
        "description": "Show what would be replaced without making changes",
        "type": "boolean"
    }
]
```

# arguments
```json
[
    {
        "name": "old-module",
        "description": "The module to replace",
        "type": "string",
        "default": ""
    },
    {
        "name": "new-module",
        "description": "The replacement module",
        "type": "string",
        "default": ""
    }
]
```

# settings
```json
{
    "description": "Replace go module in the given directory"
}
``` 