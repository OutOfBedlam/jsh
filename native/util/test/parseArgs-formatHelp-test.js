const parseArgs = require('/lib/util/parseArgs');

console.log('=== Test 1: Basic formatHelp with camelCase options ===');
const help1 = parseArgs.formatHelp({
    usage: 'Usage: myapp [options] <file>',
    options: {
        userName: { type: 'string', short: 'u', description: 'User name', default: 'guest' },
        maxRetryCount: { type: 'string', short: 'r', description: 'Maximum retry count', default: '3' },
        enableDebug: { type: 'boolean', short: 'd', description: 'Enable debug mode', default: false },
        port: { type: 'string', short: 'p', description: 'Port number', default: '8080' }
    },
    positionals: [
        { name: 'file', description: 'Input file to process' }
    ]
});
console.log(help1);
console.log();

console.log('=== Test 2: formatHelp with variadic positional ===');
const help2 = parseArgs.formatHelp({
    usage: 'Usage: command [options] <files...>',
    options: {
        outputDir: { type: 'string', short: 'o', description: 'Output directory' },
        verboseMode: { type: 'boolean', short: 'v', description: 'Verbose output' }
    },
    positionals: [
        { name: 'files', variadic: true, description: 'Files to process' }
    ]
});
console.log(help2);
console.log();

console.log('=== Test 3: toKebabCase function ===');
console.log('userName ->', parseArgs.toKebabCase('userName'));
console.log('maxRetryCount ->', parseArgs.toKebabCase('maxRetryCount'));
console.log('enableDebug ->', parseArgs.toKebabCase('enableDebug'));
console.log('port ->', parseArgs.toKebabCase('port'));
console.log('HTTPServer ->', parseArgs.toKebabCase('HTTPServer'));
