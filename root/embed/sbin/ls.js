(() => {
    const process = require('process');
    const parseArgs = require('util/parseArgs');
    const pwd = process.env.get("PWD");
    const fs = process.env.filesystem();

    // Parse command line arguments
    const { values, positionals } = parseArgs(process.argv.slice(2), {
        options: {
            long: { type: 'boolean', short: 'l', default: false },
            all: { type: 'boolean', short: 'a', default: false },
            time: { type: 'boolean', short: 't', default: false },
            recursive: { type: 'boolean', short: 'R', default: false }
        },
        strict: false,
        allowPositionals: true
    });

    // ANSI color codes
    const colors = {
        reset: "\x1b[0m",
        blue: "\x1b[34m",      // directory
        cyan: "\x1b[36m",      // symlink
        green: "\x1b[32m",     // executable
        yellow: "\x1b[33m",    // device
        magenta: "\x1b[35m",   // pipe/socket
        red: "\x1b[31m",       // archive
        white: "\x1b[37m"      // regular file
    };

    // Extract flags and directories
    const longFormat = values.long;
    const showAll = values.all;
    const sortByTime = values.time;
    const recursive = values.recursive;

    let dirs = positionals.length > 0 ? positionals : [pwd];

    var showDir = dirs.length > 1;

    // Get color for file based on mode
    let getColor = function (nfo) {
        const mode = nfo.mode();
        const modeStr = mode.string();
        const fileName = nfo.name();

        if (modeStr.startsWith("d")) {
            return colors.blue;  // directory
        } else if (modeStr.startsWith("l")) {
            return colors.cyan;  // symlink
        } else if (modeStr.startsWith("c") || modeStr.startsWith("b")) {
            return colors.yellow;  // character/block device
        } else if (modeStr.startsWith("p") || modeStr.startsWith("s")) {
            return colors.magenta;  // pipe or socket
        } else if (fileName.endsWith(".js")) {
            return colors.yellow;  // JavaScript files
        } else if (modeStr.includes("x")) {
            return colors.green;  // executable
        } else {
            return colors.white;  // regular file
        }
    };

    // Print function for detailed listing (-l)
    let printDetailed = function (nfo, idx) {
        const color = getColor(nfo);
        console.printf(`%-12s %10d %v %s%s%s\n`,
            nfo.mode().string(), nfo.size(), nfo.modTime(),
            color, nfo.name(), colors.reset);
    };

    // Print function for simple listing (no -l)
    let printSimple = function (nfo, idx) {
        const color = getColor(nfo);
        console.printf(`%s%s%s  `, color, nfo.name(), colors.reset);
    };

    let print = longFormat ? printDetailed : printSimple;

    // Helper function to filter entries
    let filterEntries = function (entries) {
        if (!showAll) {
            // Filter out hidden files (starting with .)
            entries = entries.filter((d) => !d.name().startsWith('.'));
        }

        if (sortByTime) {
            // Sort by modification time (newest first)
            entries = entries.sort((a, b) => {
                const aTime = a.modTime();
                const bTime = b.modTime();
                // Direct comparison of time objects
                if (aTime > bTime) return -1;
                if (aTime < bTime) return 1;
                return 0;
            });
        }

        return entries;
    };

    // Helper function to list directory
    let listDirectory = function (dir, prefix) {
        if (!dir.startsWith("/")) {
            dir = pwd + "/" + dir;
        }

        if (prefix) {
            console.println(dir + ":");
        } else if (showDir) {
            console.println(dir + ":");
        }

        const entries = fs.readDir(dir).map((d) => d.info());
        const filtered = filterEntries(entries);

        filtered.forEach(print);

        if (showDir || !longFormat) {
            console.println();
        }

        // Recursively list subdirectories if -R flag is set
        if (recursive) {
            filtered.forEach((nfo) => {
                if (nfo.mode().string().startsWith("d") && nfo.name() !== "." && nfo.name() !== "..") {
                    const subdir = dir + (dir.endsWith("/") ? "" : "/") + nfo.name();
                    console.println();
                    listDirectory(subdir, prefix ? prefix + "  " : "  ");
                }
            });
        }
    };

    dirs.forEach((dir) => {
        listDirectory(dir, null);
    })
})()