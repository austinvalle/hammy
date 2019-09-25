import { Client } from 'discord.js';
import { Container } from 'inversify';
import { Pool } from 'pg';
import 'reflect-metadata';
import { Stopwatch } from 'ts-stopwatch';
import * as ytdl from 'ytdl-core';

import { MessageHandler } from '../models/message-handler';
import {
	PauseMediaMessageHandler,
	PingMessageHandler,
	PlayYoutubeUrlMessageHandler,
	ResumeMediaMessageHandler,
	StopMediaMessageHandler,
	WATOBetMessageHandler,
	WATOChallengeMessageHandler,
	WATODeclineMessageHandler,
	WATOResponseMessageHandler,
} from '../modules';
import { VoiceChannelService } from '../modules/media/voice-channel.service';
import { WATODatabase } from '../modules/wato/db/wato-database';
import { StopwatchCreator, SYMBOLS, YtdlCreator } from '../types';

import { Bot } from './bot';

const container = new Container();

container.bind<Bot>(SYMBOLS.Bot).to(Bot).inSingletonScope();
container.bind<Client>(SYMBOLS.Client).toConstantValue(
	new Client({ disabledEvents: ['TYPING_START'] })
);
container.bind<string | undefined>(SYMBOLS.Token).toConstantValue(process.env.DISCORD_BOT_TOKEN);
container.bind<Pool>(SYMBOLS.DbPool).toConstantValue(new Pool());
container.bind<WATODatabase>(SYMBOLS.WATODatabase).to(WATODatabase);
container.bind<StopwatchCreator>(SYMBOLS.StopwatchCreator).toDynamicValue(() => () => new Stopwatch());
container.bind<YtdlCreator>(SYMBOLS.YtdlCreator).toDynamicValue(
	() => (url: string, opts?: ytdl.downloadOptions | undefined) => ytdl(url, opts)
);
container.bind<VoiceChannelService>(SYMBOLS.VoiceChannelService).to(VoiceChannelService);

container.bind<PingMessageHandler>(SYMBOLS.PingMessageHandler).to(PingMessageHandler);
container.bind<PlayYoutubeUrlMessageHandler>(SYMBOLS.PlayYoutubeUrlMessageHandler).to(PlayYoutubeUrlMessageHandler);
container.bind<PauseMediaMessageHandler>(SYMBOLS.PauseMediaMessageHandler).to(PauseMediaMessageHandler);
container.bind<ResumeMediaMessageHandler>(SYMBOLS.ResumeMediaMessageHandler).to(ResumeMediaMessageHandler);
container.bind<StopMediaMessageHandler>(SYMBOLS.StopMediaMessageHandler).to(StopMediaMessageHandler);
container.bind<WATOChallengeMessageHandler>(SYMBOLS.WATOChallengeMessageHandler).to(WATOChallengeMessageHandler);
container.bind<WATOResponseMessageHandler>(SYMBOLS.WATOResponseMessageHandler).to(WATOResponseMessageHandler);
container.bind<WATODeclineMessageHandler>(SYMBOLS.WATODeclineMessageHandler).to(WATODeclineMessageHandler);
container.bind<WATOBetMessageHandler>(SYMBOLS.WATOBetMessageHandler).to(WATOBetMessageHandler);

container.bind<MessageHandler[]>(SYMBOLS.MessageHandlers).toConstantValue(_createHandlers());

export default container;

function _createHandlers(): MessageHandler[] {
	return [
		container.get<PingMessageHandler>(SYMBOLS.PingMessageHandler),
		container.get<PlayYoutubeUrlMessageHandler>(SYMBOLS.PlayYoutubeUrlMessageHandler),
		container.get<PauseMediaMessageHandler>(SYMBOLS.PauseMediaMessageHandler),
		container.get<ResumeMediaMessageHandler>(SYMBOLS.ResumeMediaMessageHandler),
		container.get<StopMediaMessageHandler>(SYMBOLS.StopMediaMessageHandler),
		container.get<WATOChallengeMessageHandler>(SYMBOLS.WATOChallengeMessageHandler),
		container.get<WATOResponseMessageHandler>(SYMBOLS.WATOResponseMessageHandler),
		container.get<WATODeclineMessageHandler>(SYMBOLS.WATODeclineMessageHandler),
		container.get<WATOBetMessageHandler>(SYMBOLS.WATOBetMessageHandler)
	];
}
