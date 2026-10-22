import { generateKeyPairSync } from 'node:crypto';

const { privateKey, publicKey } = generateKeyPairSync('ec', {
  namedCurve: 'prime256v1',
  publicKeyEncoding: { format: 'jwk' },
  privateKeyEncoding: { format: 'jwk' },
});
const decode = (value) => Buffer.from(value, 'base64url');
const publicBytes = Buffer.concat([Buffer.from([4]), decode(publicKey.x), decode(publicKey.y)]);
console.log(`PUSH_VAPID_PUBLIC_KEY=${publicBytes.toString('base64url')}`);
console.log(`PUSH_VAPID_PRIVATE_KEY=${privateKey.d}`);
console.log('PUSH_VAPID_SUBJECT=admin@openchore.local');
