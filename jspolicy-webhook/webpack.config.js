const path = require('path')
const glob = require('glob');
const { PolicyBundlePlugin } = require('@jspolicy/webpack-plugin');
const tsconfig = require('./tsconfig.json');

module.exports = {
    mode: 'production',
    entry: glob.sync('./dist/**/index.js').reduce((acc, path) => {
        const entry = path.replace('/index.js', '')
        acc[entry] = './' + path
        return acc
    }, {}),
    output: {
        filename: './[name]/bundle.js',
        path: path.resolve(__dirname),
        libraryTarget: 'commonjs',
    },
    plugins: [
        new PolicyBundlePlugin({
            tsOutDir: tsconfig.compilerOptions.outDir
        }),
    ],
};