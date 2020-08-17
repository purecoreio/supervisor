const colors = require('colors');
const inquirer = require('inquirer')

class ConsoleUtil {


    public static loadingInterval = null;
    public static loadingStep = 0;
    public static character = null;

    public static showTitle() {

        console.log(colors.white("                                                         _                "))
        console.log(colors.white("                                                        (_)               "))
        console.log(colors.white("   ___ ___  _ __ ___     ___ _   _ _ __   ___ _ ____   ___ ___  ___  _ __ "))
        console.log(colors.white("  / __/ _ \\| '__/ _ \\   / __| | | | '_ \\ / _ \\ '__\\ \\ / / / __|/ _ \\| '__|"))
        console.log(colors.white(" | (_| (_) | | |  __/_  \\__ \\ |_| | |_) |  __/ |   \\ V /| \\__ \\ (_) | |   "))
        console.log(colors.white("  \\___\\___/|_|  \\___(_) |___/\\__,_| .__/ \\___|_|    \\_/ |_|___/\\___/|_|   "))
        console.log(colors.white("                                  | |                                     "))
        console.log(colors.white("                                  |_|                                     "))
        console.log("")
        console.log(colors.white("     ◢ by © quiquelhappy "))
        console.log("")
        console.log("")
    }

    public static askHash(): Promise<string> {
        var questions = [{
            type: 'password',
            name: 'hash',
            prefix: colors.bgMagenta(" ? "),
            message: colors.magenta("Please, enter your machine hash and press enter"),
        }]
        return new Promise(function (resolve, reject) {
            inquirer.prompt(questions).then(answers => {
                resolve(answers['hash']);
            }).catch(() => {
                reject();
            })
        });

    }

    public static setLoading(loading, string, failed = false) {

        if (loading) {

            ConsoleUtil.loadingInterval = setInterval(() => {

                ConsoleUtil.loadingStep++

                process.stdout.clearLine(0)
                process.stdout.cursorTo(0)

                switch (ConsoleUtil.loadingStep) {
                    case 1:
                        ConsoleUtil.character = " ▖ "
                        break;
                    case 2:
                        ConsoleUtil.character = " ▘ "
                        break;
                    case 3:
                        ConsoleUtil.character = " ▝ "
                        break;
                    default:
                        ConsoleUtil.character = " ▗ "
                        ConsoleUtil.loadingStep = 0
                        break;
                }

                process.stdout.write(colors.bgMagenta(ConsoleUtil.character) + colors.magenta(" " + string))

            }, 100);
        } else {

            clearInterval(ConsoleUtil.loadingInterval);
            process.stdout.clearLine(0)
            process.stdout.cursorTo(0)

            if (!failed) {
                process.stdout.write(colors.bgGreen(" ✓ ") + colors.green(" " + string))
            } else {
                process.stdout.write(colors.bgRed(" ☓ ") + colors.red(" " + string))
            }

            process.stdout.write("\n");
            process.stdout.cursorTo(0)

        }

    }

}

module.exports.ConsoleUtil = ConsoleUtil;