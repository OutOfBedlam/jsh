var optparse = require('optparse');

var SWITCHES = [
    ['-h', '--help', 'Show this help message'],
    ['-v', '--version', 'Show version information'],
];

var parser = new optparse.OptionParser(SWITCHES);

parser.banner = 'Usage: command [options]';

var options = {
    version: false,
    help: false,
};

parser.on('version', function() {
    console.println('command version 0.1.0');
    options.version = true;
});

parser.on('help', function() {
    console.println(parser.toString());
    options.help = true;
});

parser.parse(runtime.args);

console.println('Options:', `{help:${options.help}, version:${options.version}}`);