(() => {
    const pwd = runtime.env.get("PWD");
    const fs = runtime.env.filesystem();
    const args = runtime.args;

    var dirs = args.length === 0 ? [pwd] : args;
    var showDir = dirs.length > 1;

    let print = function (nfo, idx) {
        // nfo.sys() => *syscall.Stat_t
        console.printf(`%-12s %10d %v %s\n`,
            nfo.mode().string(), nfo.size(), nfo.modTime(), nfo.name());
    }
    dirs.forEach((dir) => {
        if (!dir.startsWith("/")) {
            dir = pwd + "/" + dir;
        }
        if (showDir) {
            console.println(dir + ":");
        }
        fs.readDir(dir).map((d) => d.info()).forEach(print);
        if (showDir) {
            console.println();
        }
    })
})()