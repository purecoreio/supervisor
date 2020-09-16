const io = require('socket.io-client');
class SocketClient {

    public static socket;

    public setup() {
        SocketClient.socket = io.connect('https://socket.purecore.io/', { path: "/hosting" });

        /**
         * Authentication process:
         * 1. connect
         * 2. authenticate with hash
         * 3. set as host machine when authenticated
         */
        SocketClient.socket.on('connect', function () {
            Supervisor.emitter.emit('socketConnected')
            Supervisor.emitter.emit('socketAuthenticating')
            SocketClient.socket.emit('auth', Supervisor.hash)
        });
        SocketClient.socket.on('authenticated', function (data) {
            Supervisor.emitter.emit('socketAuthenticated')
            Supervisor.emitter.emit('socketHostRequest')
            SocketClient.socket.emit('host')
        });
        SocketClient.socket.on('hosting', function (data) {
            Supervisor.emitter.emit('socketHosting')
        });

        /**
         * Admin Events
         */
        SocketClient.socket.on('createContainer', function (data) {
            console.log("create container " + data.host.uuid);
        });

        /**
         * User Events
         */
        SocketClient.socket.on('restartContainer', function (data) {
            console.log("restartContainer " + data.host.uuid);
        });
        SocketClient.socket.on('startContainer', function (data) {
            console.log("startContainer " + data.host.uuid);
        });
        SocketClient.socket.on('stopContainer', function (data) {
            console.log("stopContainer " + data.host.uuid);
        });
        SocketClient.socket.on('pauseContainer', function (data) {
            console.log("pauseContainer " + data.host.uuid);
        });
        SocketClient.socket.on('resumeContainer', function (data) {
            console.log("resumeContainer " + data.host.uuid);
        });

        /** 
         *  Status Updates
         */
        SocketClient.socket.on('disconnect', function () {
            Supervisor.emitter.emit('socketDisconnected')
        });
        SocketClient.socket.on('error', function () {
            Supervisor.emitter.emit('socketError')
        });
        SocketClient.socket.on('reconnect', function () {
            Supervisor.emitter.emit('socketReconnected')
        });
        SocketClient.socket.on('reconnecting', function () {
            Supervisor.emitter.emit('socketReconnecting')
        });
        SocketClient.socket.on('reconnect_error', function () {
            Supervisor.emitter.emit('socketReconnectingError')
        });
        SocketClient.socket.on('reconnect_failed', function () {
            Supervisor.emitter.emit('socketReconnectFailed')
        });
    }

}