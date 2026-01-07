# parseArgs

A Node.js-compatible command-line argument parser for JSH runtime, compatible with Node.js `util.parseArgs()` API.

## Installation

```javascript
const { parseArgs } = require('/lib/util');
```

## Usage

### Basic Example

```javascript
const { parseArgs } = require('/lib/util');

const args = ['-f', '--bar', 'value', 'positional'];
const result = parseArgs(args, {
    options: {
        foo: { type: 'boolean', short: 'f' },
        bar: { type: 'string' }
    },
    allowPositionals: true
});

console.log(result.values);      // { foo: true, bar: 'value' }
console.log(result.positionals); // ['positional']
```

## API

### Function Signatures

`parseArgs()` supports two call signatures:

#### New Signature (Recommended)
```javascript
parseArgs(args, config)
```

- `args` (Array, optional): Array of strings to parse. Defaults to `process.argv.slice(2)` if omitted.
- `config` (Object): Configuration object

#### Legacy Signature
```javascript
parseArgs(config)
```

- `config` (Object): Configuration object with `args` property

### Configuration Object

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `options` | Object | `{}` | Option definitions (see below) |
| `strict` | boolean | `true` | Throw error on unknown options |
| `allowPositionals` | boolean | `!strict` | Allow positional arguments |
| `allowNegative` | boolean | `false` | Allow `--no-` prefix for boolean options |
| `tokens` | boolean | `false` | Return detailed parsing tokens |
| `positionals` | Array | `undefined` | Named positional arguments definition (see below) |

### Option Definition

Each option in the `options` object can have:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | Yes | One of: `'boolean'`, `'string'`, `'integer'`, or `'float'` |
| `short` | string | No | Single character short option (e.g., `'f'` for `-f`) |
| `multiple` | boolean | No | Allow option to be specified multiple times (collects values in array) |
| `default` | any | No | Default value if option is not provided |

**Supported Types:**
- `'boolean'`: True/false flag, does not take a value
- `'string'`: Accepts any string value
- `'integer'`: Parses value as integer, validates that no decimal point is present
- `'float'`: Parses value as floating-point number, allows decimal values

**Note:** Option names automatically convert from camelCase to kebab-case for CLI flags. For example, `userName` becomes `--user-name`, allowing you to use JavaScript naming conventions in code while following Linux CLI conventions on the command line.

### Positional Definition

The `positionals` array allows you to assign names to positional arguments. Each element can be:

**String format** (simple):
```javascript
positionals: ['inputFile', 'outputFile']
```

**Object format** (advanced):

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `name` | string | Required | Name for this positional argument |
| `optional` | boolean | `false` | Whether this argument is optional |
| `default` | any | `undefined` | Default value if not provided (requires `optional: true`) |
| `variadic` | boolean | `false` | Collect all remaining arguments (must be last) |

**Important Rules:**
- Variadic positionals must be the last in the array
- Required positionals cannot come after optional ones
- Missing required positionals will throw an error

### Return Value

Returns an object with:

- `values` (Object): Parsed option values
- `positionals` (Array): Positional arguments (always present)
- `namedPositionals` (Object, optional): Named positional arguments (only if `positionals` config provided)
- `tokens` (Array, optional): Detailed parsing information (only if `tokens: true`)

## Examples

### CamelCase to Kebab-Case Conversion

Option names are automatically converted from camelCase to kebab-case for CLI flags:

```javascript
const result = parseArgs(['--user-name', 'Alice', '--max-retry-count', '5', '--enable-debug'], {
    options: {
        userName: { type: 'string' },           // Becomes --user-name
        maxRetryCount: { type: 'string' },      // Becomes --max-retry-count
        enableDebug: { type: 'boolean' }        // Becomes --enable-debug
    }
});
// result.values: { userName: 'Alice', maxRetryCount: '5', enableDebug: true }
```

This allows you to use JavaScript naming conventions (camelCase) in your code while following traditional Linux CLI conventions (kebab-case) on the command line. Simple names without capital letters (like `port`, `verbose`) are not converted.

### Long Options

```javascript
const result = parseArgs(['--verbose', '--output', 'file.txt'], {
    options: {
        verbose: { type: 'boolean' },
        output: { type: 'string' }
    }
});
// result.values: { verbose: true, output: 'file.txt' }
```

### Short Options

```javascript
const result = parseArgs(['-v', '-o', 'out.txt'], {
    options: {
        verbose: { type: 'boolean', short: 'v' },
        output: { type: 'string', short: 'o' }
    }
});
// result.values: { verbose: true, output: 'out.txt' }
```

### Integer and Float Options

Use `integer` type for whole numbers and `float` type for decimal numbers:

```javascript
const result = parseArgs(['--port', '8080', '--ratio', '0.75', '-c', '10'], {
    options: {
        port: { type: 'integer' },           // Parses as integer
        ratio: { type: 'float' },            // Parses as float
        count: { type: 'integer', short: 'c' }
    }
});
// result.values: { port: 8080, ratio: 0.75, count: 10 }
// All numeric values are JavaScript numbers (typeof === 'number')
```

**Integer validation:**
```javascript
// This will throw an error because 3.14 contains a decimal point
parseArgs(['--count', '3.14'], {
    options: { count: { type: 'integer' } }
});
// TypeError: Option --count requires an integer value, got: 3.14

// This will throw an error because 'abc' is not a valid number
parseArgs(['--port', 'abc'], {
    options: { port: { type: 'integer' } }
});
// TypeError: Option --port requires a valid integer value, got: abc
```

### Inline Values

Supports both `--option=value` and `-o=value` formats:

```javascript
const result = parseArgs(['--output=file.txt', '-o=out.txt'], {
    options: {
        output: { type: 'string', short: 'o' }
    }
});
// result.values: { output: 'out.txt' } // Last value wins
```

### Multiple Values

Collect multiple values for the same option:

```javascript
const result = parseArgs(['--include', 'a.js', '--include', 'b.js', '-I', 'c.js'], {
    options: {
        include: { type: 'string', short: 'I', multiple: true }
    }
});
// result.values: { include: ['a.js', 'b.js', 'c.js'] }
```

### Default Values

```javascript
const result = parseArgs(['--foo'], {
    options: {
        foo: { type: 'boolean' },
        bar: { type: 'string', default: 'default_value' },
        count: { type: 'string', default: '0' }
    }
});
// result.values: { foo: true, bar: 'default_value', count: '0' }
```

### Short Option Groups

Bundle multiple boolean short options together:

```javascript
const result = parseArgs(['-abc'], {
    options: {
        a: { type: 'boolean', short: 'a' },
        b: { type: 'boolean', short: 'b' },
        c: { type: 'boolean', short: 'c' }
    }
});
// result.values: { a: true, b: true, c: true }
```

### Option Terminator

Use `--` to separate options from positional arguments:

```javascript
const result = parseArgs(['--foo', '--', '--bar', 'baz'], {
    options: {
        foo: { type: 'boolean' },
        bar: { type: 'boolean' }
    },
    allowPositionals: true
});
// result.values: { foo: true }
// result.positionals: ['--bar', 'baz']
```

### Negative Options

Enable `--no-` prefix to set boolean options to false. Works with camelCase option names:

```javascript
const result = parseArgs(['--no-color', '--verbose', '--no-enable-debug'], {
    options: {
        color: { type: 'boolean' },
        verbose: { type: 'boolean' },
        enableDebug: { type: 'boolean' }      // --no-enable-debug sets to false
    },
    allowNegative: true
});
// result.values: { color: false, verbose: true, enableDebug: false }
```

### Named Positionals

Assign names to positional arguments for easier access:

```javascript
const result = parseArgs(['input.txt', 'output.txt'], {
    options: {},
    allowPositionals: true,
    positionals: ['inputFile', 'outputFile']
});
// result.positionals: ['input.txt', 'output.txt']
// result.namedPositionals: { inputFile: 'input.txt', outputFile: 'output.txt' }
```

### Optional Positionals

Make positional arguments optional with default values:

```javascript
const result = parseArgs(['input.txt'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'inputFile',
        { name: 'outputFile', optional: true, default: 'stdout' }
    ]
});
// result.positionals: ['input.txt']
// result.namedPositionals: { inputFile: 'input.txt', outputFile: 'stdout' }
```

### Variadic Positionals

Collect remaining arguments into an array:

```javascript
const result = parseArgs(['input.txt', 'output.txt', 'file1.js', 'file2.js', 'file3.js'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'inputFile',
        'outputFile',
        { name: 'files', variadic: true }
    ]
});
// result.positionals: ['input.txt', 'output.txt', 'file1.js', 'file2.js', 'file3.js']
// result.namedPositionals: {
//     inputFile: 'input.txt',
//     outputFile: 'output.txt',
//     files: ['file1.js', 'file2.js', 'file3.js']
// }
```

### Named Positionals with Options

Combine options and named positionals:

```javascript
const result = parseArgs(['-v', '--config', 'app.json', 'src.js', 'dest.js'], {
    options: {
        verbose: { type: 'boolean', short: 'v' },
        config: { type: 'string' }
    },
    allowPositionals: true,
    positionals: ['source', 'destination']
});
// result.values: { verbose: true, config: 'app.json' }
// result.positionals: ['src.js', 'dest.js']
// result.namedPositionals: { source: 'src.js', destination: 'dest.js' }
```

### Tokens Mode

Get detailed parsing information:

```javascript
const result = parseArgs(['-f', '--bar', 'value'], {
    options: {
        foo: { type: 'boolean', short: 'f' },
        bar: { type: 'string' }
    },
    tokens: true
});

// result.tokens: [
//   { kind: 'option', name: 'foo', rawName: '-f', index: 0, value: undefined, inlineValue: undefined },
//   { kind: 'option', name: 'bar', rawName: '--bar', index: 1, value: 'value', inlineValue: false }
// ]
```

### Token Structure

Each token has:

- **All tokens:**
  - `kind`: `'option'`, `'positional'`, or `'option-terminator'`
  - `index`: Position in the args array

- **Option tokens:**
  - `name`: Long option name
  - `rawName`: How the option was specified (e.g., `-f`, `--foo`)
  - `value`: Option value (undefined for boolean options)
  - `inlineValue`: Whether value was specified inline (e.g., `--foo=bar`)

- **Positional tokens:**
  - `value`: The positional argument value

## Error Handling

The parser throws `TypeError` in the following cases:

- Unknown option when `strict: true` (default)
- Missing value for string, integer, or float option
- Invalid number format for integer or float option
- Decimal value for integer option (e.g., `3.14` when integer is expected)
- Unexpected positional argument when `allowPositionals: false` and `strict: true`
- Using `--no-` prefix on non-boolean option when `strict: true`
- Boolean option with inline value when `strict: true`
- Missing required positional argument (when using named positionals)
- Variadic positional not in last position (when using named positionals)

### Example

```javascript
try {
    const result = parseArgs(['--unknown'], {
        options: {},
        strict: true
    });
} catch (error) {
    console.error(error.message); // "Unknown option: --unknown"
}
```

```javascript
// Missing required positional
try {
    parseArgs(['input.txt'], {
        positionals: ['inputFile', 'outputFile']  // outputFile required
    });
} catch (error) {
    console.error(error.message); // "Missing required positional argument: outputFile"
}
```

```javascript
// Variadic not last
try {
    parseArgs([], {
        positionals: [
            { name: 'files', variadic: true },
            'output'  // Error: cannot come after variadic
        ]
    });
} catch (error) {
    console.error(error.message); // "Variadic positional argument must be the last argument"
}
```

## Compatibility

This implementation is compatible with Node.js `util.parseArgs()` API, supporting:

- ✅ Long options (`--option`)
- ✅ Short options (`-o`)
- ✅ Short option groups (`-abc`)
- ✅ Inline values (`--option=value`, `-o=value`)
- ✅ Boolean and string types
- ✅ Multiple values
- ✅ Default values
- ✅ Positional arguments
- ✅ Option terminator (`--`)
- ✅ Negative options (`--no-option`)
- ✅ Strict mode
- ✅ Tokens mode

## Extended Features

Beyond Node.js `util.parseArgs()`, this implementation adds:

- ✅ **Integer and float types** - Numeric types with automatic parsing and validation
- ✅ **Named positionals** - Assign names to positional arguments
- ✅ **Optional positionals** - Make positional arguments optional with defaults
- ✅ **Variadic positionals** - Collect remaining arguments into an array
- ✅ **Positional validation** - Automatic validation of required arguments
- ✅ **CamelCase to kebab-case conversion** - Automatically converts option names from camelCase to kebab-case for CLI flags

## License

See project license.
