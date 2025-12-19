const {parseArgs} = require('/lib/util');

const args = ['-f', '--bar', 'value', 'positional'];
const options = {
  foo: { type: 'boolean', short: 'f' },
  bar: { type: 'string' }
};

const { values, positionals } = parseArgs(args, { options, allowPositionals: true });
console.println({ values, positionals });
