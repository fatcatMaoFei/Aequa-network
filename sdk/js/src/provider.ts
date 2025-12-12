import fetch from "node-fetch";

export type TxType = "plaintext_v1" | "auction_bid_v1" | "private_v1";

export interface TxEnvelope {
  type?: TxType; // defaults to plaintext_v1
  from: string;
  nonce: number;
  gas?: number;
  fee?: number;
  bid?: number;
  fee_recipient?: string;
  ciphertext?: string; // base64
  ephemeral_key?: string; // base64
  target_height?: number;
  sig?: string; // base64
}

export class AequaProvider {
  constructor(private baseUrl: string) {}

  async submitTx(tx: TxEnvelope): Promise<void> {
    const env = { ...tx };
    if (!env.type) env.type = "plaintext_v1";
    const resp = await fetch(`${this.baseUrl}/v1/tx/plain`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(env),
    });
    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`submitTx failed: ${resp.status} ${text}`);
    }
  }

  async getDuty(): Promise<any> {
    const resp = await fetch(`${this.baseUrl}/v1/duty`, { method: "GET" });
    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`getDuty failed: ${resp.status} ${text}`);
    }
    return resp.json();
  }
}
