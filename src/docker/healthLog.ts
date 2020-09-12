class HealthLog {

    public host: string;
    public lastLog: any;
    public emitter: any;

    public constructor(host: string, log: any) {
        this.host = host;
        this.lastLog = log;
        this.emitter = new EventEmitter();
    }

    public pushLog(log) {
        log = JSON.parse(log.toString('utf8'));
        this.emitter.emit('log', log);
        this.lastLog = {
            time: Date.now(),
            log: log,
        }
        this.lastLog = log;
    }

}