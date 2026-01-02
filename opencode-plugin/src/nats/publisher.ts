import type {
	CodingResult,
	CodingStreamEvent,
	StreamEventType,
} from "../types";
import type { NATSClient } from "./client";

export class EventPublisher {
	private nats: NATSClient | null;
	private taskID: string;
	private streamSubject: string;
	private resultSubject: string;
	private seq = 0;

	constructor(
		nats: NATSClient | null,
		taskID: string,
		streamSubject: string,
		resultSubject: string,
	) {
		this.nats = nats;
		this.taskID = taskID;
		this.streamSubject = streamSubject;
		this.resultSubject = resultSubject;
	}

	async stream(
		type: StreamEventType,
		data: Partial<
			Omit<CodingStreamEvent, "taskID" | "seq" | "timestamp" | "type">
		>,
	): Promise<void> {
		this.seq++;

		const event: CodingStreamEvent = {
			taskID: this.taskID,
			seq: this.seq,
			timestamp: new Date().toISOString(),
			type,
			...data,
		};

		if (this.nats) {
			await this.nats.publish(this.streamSubject, JSON.stringify(event));
		}

		console.log(`[station-plugin] Stream event #${this.seq}: ${type}`);
	}

	async result(data: Omit<CodingResult, "taskID">): Promise<void> {
		const result: CodingResult = {
			taskID: this.taskID,
			...data,
		};

		if (this.nats) {
			await this.nats.publish(this.resultSubject, JSON.stringify(result));
		}

		console.log(`[station-plugin] Result: ${result.status}`);
	}

	getEventCount(): number {
		return this.seq;
	}
}
