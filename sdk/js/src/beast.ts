import { randomBytes, createCipheriv, createHash } from "crypto";
import { TxEnvelope, TxType } from "./provider";

export type PrivateInnerType = Exclude<TxType, "private_v1">;

export interface PrivateInnerTx {
  type?: PrivateInnerType;
  from: string;
  nonce: number;
  gas?: number;
  fee?: number;
  bid?: number;
  fee_recipient?: string;
}

export interface EncryptPrivateParams {
  groupPubKey: Uint8Array | string;
  targetHeight: number;
  tx: PrivateInnerTx;
}

function deriveKey(groupPubKey: Uint8Array | string): Buffer {
  let raw: Buffer;
  if (typeof groupPubKey === "string") {
    const trimmed = groupPubKey.trim();
    if (/^0x[0-9a-fA-F]+$/.test(trimmed)) {
      raw = Buffer.from(trimmed.slice(2), "hex");
    } else {
      raw = Buffer.from(trimmed, "base64");
    }
  } else {
    raw = Buffer.from(groupPubKey);
  }
  return createHash("sha256").update(raw).digest();
}

// encryptPrivateTx builds a private_v1 TxEnvelope compatible with the Go BEAST
// symmetric engine (AES-GCM with key = sha256(groupPubKey)).
export function encryptPrivateTx(params: EncryptPrivateParams): TxEnvelope {
  const { groupPubKey, targetHeight, tx } = params;
  const innerType: PrivateInnerType = tx.type ?? "plaintext_v1";

  const key = deriveKey(groupPubKey);
  const iv = randomBytes(12); // matches Go cipher.NewGCM nonce size

  const envelope = {
    type: innerType,
    from: tx.from,
    nonce: tx.nonce,
    gas: tx.gas ?? 0,
    fee: tx.fee ?? 0,
    bid: tx.bid ?? 0,
    fee_recipient: tx.fee_recipient ?? "",
  };
  const plaintext = Buffer.from(JSON.stringify(envelope), "utf8");

  const cipher = createCipheriv("aes-256-gcm", key, iv);
  const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const tag = cipher.getAuthTag();
  const full = Buffer.concat([iv, ciphertext, tag]);

  const eph = randomBytes(32);

  const env: TxEnvelope = {
    type: "private_v1",
    from: tx.from,
    nonce: tx.nonce,
    ciphertext: full.toString("base64"),
    ephemeral_key: eph.toString("base64"),
    target_height: targetHeight,
  };
  return env;
}

