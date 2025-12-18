(() => {
    const process = require("process");
    const pwd = process.env.get("PWD");
    const fs = process.env.filesystem();
    const args = process.argv.slice(2);

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

    // Parse options and directories
    let longFormat = false;
    let dirs = [];
    
    args.forEach((arg) => {
        if (arg === "-l" || arg === "-al" || arg === "-la") {
            longFormat = true;
        } else {
            dirs.push(arg);
        }
    });

    if (dirs.length === 0) {
        dirs = [pwd];
    }

    var showDir = dirs.length > 1;

    // Get color for file based on mode
    let getColor = function(nfo) {
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

    dirs.forEach((dir) => {
        if (!dir.startsWith("/")) {
            dir = pwd + "/" + dir;
        }
        if (showDir) {
            console.println(dir + ":");
        }
        fs.readDir(dir).map((d) => d.info()).forEach(print);
        if (showDir || !longFormat) {
            console.println();
        }
    })
})()