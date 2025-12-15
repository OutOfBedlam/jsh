(() => {
    const {args} = require('process');
    if (args.length === 0) {
        console.println();
        return;
    }

    // args = args.map(arg => {
    //     if (typeof arg === 'string') {
    //         return arg;
    //     } else if (arg === null || arg === undefined) {
    //         return '';
    //     } else {
    //         return String(arg);
    //     }
    // });

    console.println(...args);
})()
