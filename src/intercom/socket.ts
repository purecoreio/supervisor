const http = require('http');
const https = require('https');
const socketio = require('socket.io');
const app = require('express')();

class SocketServer {

    public static io;

    public getSocket(server) {
        return new socketio(server).on('connection', client => {
            Supervisor.emitter.emit('clientConnected');
            client.on('event', data => { /* … */ });
            client.on('disconnect', () => { Supervisor.emitter.emit('clientDisconnected'); });
        })
    }

    public setup() {
        try {
            Supervisor.emitter.emit('creatingServer');
            const server = this.getHTTP();
            Supervisor.emitter.emit('createdServer');

            try {
                Supervisor.emitter.emit('creatingSocketServer');
                SocketServer.io = this.getSocket(server);
                server.listen(31518, () => {
                    Supervisor.emitter.emit('createdSocketServer');
                }).on('error', function (error) {
                    Supervisor.emitter.emit('errorCreatingSocketServer', new Error(error.code));
                });
            } catch (error) {
                Supervisor.emitter.emit('errorCreatingSocketServer', error);
            }
        } catch (error) {
            Supervisor.emitter.emit('errorCreatingServer', error);
        }
    }

    public getHTTP(): any {
        let httpServer = null;
        const cert = this.getCert();

        if (cert != null) {
            Supervisor.emitter.emit('certUse');
            httpServer = https.Server({
                key: cert.key,
                cert: cert.cert,
                ca: cert.ca
            }, app);
        } else {
            Supervisor.emitter.emit('certUnknown');
        }

        if (httpServer == null) {
            httpServer = http.Server(app);
        }
        return httpServer;
    }

    public getCert(): Cert {
        Supervisor.emitter.emit('certLoading');
        const fs = require("fs");
        const letsencryptBase = "/etc/letsencrypt/"
        try {
            if (fs.existsSync(letsencryptBase) && fs.existsSync(letsencryptBase + "live/")) {
                var folders = fs.readdirSync(letsencryptBase + "live/").filter((dirent) => dirent.isDirectory());
                if (folders != null && Array.isArray(folders) && folders.length > 0) {
                    Supervisor.emitter.emit('certFound');
                    return new Cert(fs.readFileSync(letsencryptBase + "live/" + folders[0].name + "/privkey.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0].name + "/cert.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0].name + "/chain.pem"));
                } else {
                    Supervisor.emitter.emit('certNotSetup');
                    return null;
                }
            } else {
                Supervisor.emitter.emit('certNotInstalled');
                return null;
            }
        } catch (error) {
            Supervisor.emitter.emit('certReadingError');
            return null;
        }
    }

}