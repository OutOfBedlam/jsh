(() => {
    const env = require('/lib/process').env;
    const args = require('/lib/process').argv.slice(2);
    if (args.length === 0) {
        console.println();
        return;
    }

    // Substitute environment variables in the string
    function substituteEnvVars(str) {
        // Replace ${VAR} format
        let result = str.replace(/\$\{([A-Za-z_][A-Za-z0-9_]*)\}/g, (match, varName) => {
            const value = env.get(varName);
            return value !== undefined ? value : "";
        });
        
        // Replace $VAR format
        result = result.replace(/\$([A-Za-z_][A-Za-z0-9_]*)/g, (match, varName) => {
            const value = env.get(varName);
            return value !== undefined ? value : "";
        });
        
        return result;
    }

    output = [];
    for (let i = 0; i < args.length; i++) {
        output.push(substituteEnvVars(args[i]));
    }

    console.println(...output);
})()
