var sinon = require('sinon');

const ping = require('../src/modules/ping');

const sandbox = sinon.createSandbox();

describe('ping module', () => {
    var sendSpy,
        message;

    beforeEach(() => {
        sendSpy = sinon.spy();
        message = {channel: {send: sendSpy}};
    });

    describe('when send ping is called', () => {
        it('responds to the message creator with a pong paddle', () => {
            ping.sendPing(message);

            sinon.assert.calledOnce(sendSpy);
            sinon.assert.calledWith(sendSpy, ':ping_pong: Pong!');
        });
    });
});