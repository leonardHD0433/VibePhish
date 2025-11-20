const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

// Read JWT_SECRET from .env file
const envPath = path.join(__dirname, '.env');
const envContent = fs.readFileSync(envPath, 'utf8');

// Parse JWT_SECRET from .env
const jwtSecretMatch = envContent.match(/JWT_SECRET[=\s]+"?([^"\n]+)"?/);
if (!jwtSecretMatch) {
  console.error('ERROR: JWT_SECRET not found in .env file');
  process.exit(1);
}

const JWT_SECRET = jwtSecretMatch[1];

// JWT header (HS256 algorithm)
const header = {
  alg: 'HS256',
  typ: 'JWT'
};

// JWT payload
const now = Math.floor(Date.now() / 1000);
const oneYearFromNow = now + (365 * 24 * 60 * 60); // 1 year in seconds

const payload = {
  sub: 'n8n',
  iat: now,
  exp: oneYearFromNow
};

// Base64URL encode function
function base64UrlEncode(str) {
  return Buffer.from(str)
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

// Encode header and payload
const encodedHeader = base64UrlEncode(JSON.stringify(header));
const encodedPayload = base64UrlEncode(JSON.stringify(payload));

// Create signature
const signatureInput = `${encodedHeader}.${encodedPayload}`;
const signature = crypto
  .createHmac('sha256', JWT_SECRET)
  .update(signatureInput)
  .digest('base64')
  .replace(/\+/g, '-')
  .replace(/\//g, '_')
  .replace(/=/g, '');

// Combine to create JWT
const jwt = `${encodedHeader}.${encodedPayload}.${signature}`;

// Print results
console.log('='.repeat(80));
console.log('JWT TOKEN GENERATED');
console.log('='.repeat(80));
console.log('');
console.log('Token (copy this entire string):');
console.log(jwt);
console.log('');
console.log('='.repeat(80));
console.log('TOKEN DETAILS');
console.log('='.repeat(80));
console.log('Subject:', payload.sub);
console.log('Issued At:', new Date(now * 1000).toISOString());
console.log('Expires:', new Date(oneYearFromNow * 1000).toISOString());
console.log('Valid For: 1 year');
console.log('');
console.log('='.repeat(80));
console.log('N8N CONFIGURATION');
console.log('='.repeat(80));
console.log('1. In n8n, go to Settings → Environments');
console.log('2. Add a new environment variable:');
console.log('   Name: JWT_TOKEN');
console.log('   Value: (paste the token above)');
console.log('');
console.log('3. In your HTTP Request node, use this Authorization header:');
console.log('   Name: Authorization');
console.log('   Value: Bearer {{$env.JWT_TOKEN}}');
console.log('');
console.log('='.repeat(80));
