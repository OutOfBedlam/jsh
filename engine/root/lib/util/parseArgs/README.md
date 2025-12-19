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

### Option Definition

Each option in the `options` object can have:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | Yes | Either `'boolean'` or `'string'` |
| `short` | string | No | Single character short option (e.g., `'f'` for `-f`) |
| `multiple` | boolean | No | Allow option to be specified multiple times (collects values in array) |
| `default` | any | No | Default value if option is not provided |

### Return Value

Returns an object with:

- `values` (Object): Parsed option values
- `positionals` (Array): Positional arguments
- `tokens` (Array, optional): Detailed parsing information (only if `tokens: true`)

## Examples

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

Enable `--no-` prefix to set boolean options to false:

```javascript
const result = parseArgs(['--no-color', '--verbose'], {
    options: {
        color: { type: 'boolean' },
        verbose: { type: 'boolean' }
    },
    allowNegative: true
});
// result.values: { color: false, verbose: true }
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
- Missing value for string option
- Unexpected positional argument when `allowPositionals: false` and `strict: true`
- Using `--no-` prefix on non-boolean option when `strict: true`
- Boolean option with inline value when `strict: true`

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

## License

See project license.
