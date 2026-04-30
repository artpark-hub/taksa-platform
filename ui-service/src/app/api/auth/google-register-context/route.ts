import { NextRequest, NextResponse } from 'next/server';

export const runtime = 'nodejs';

const COOKIE_NAME = 'taksa_google_register_ctx';
const MAX_AGE_SECONDS = 10 * 60;
const MAX_AGE_MS = MAX_AGE_SECONDS * 1000;

type PendingContext = {
    organization_name: string;
    tenant_id: string;
    role: string;
    source: 'google';
    created_at: number;
};

let devFallbackSecret: string | undefined;
let hasWarnedAboutDevFallback = false;

const createFallbackSecret = () => {
    const bytes = new Uint8Array(32);
    globalThis.crypto.getRandomValues(bytes);
    return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
};

const getSecret = (): string | undefined => {
    const configuredSecret = process.env.TAKSA_UI_CONTEXT_SECRET || process.env.NEXTAUTH_SECRET;
    if (configuredSecret) return configuredSecret;

    if (!devFallbackSecret) {
        devFallbackSecret = createFallbackSecret();
    }

    if (!hasWarnedAboutDevFallback) {
        console.warn(
            'Using generated fallback for Google register context secret. Set TAKSA_UI_CONTEXT_SECRET or NEXTAUTH_SECRET for deterministic behavior across restarts/instances.'
        );
        hasWarnedAboutDevFallback = true;
    }

    return devFallbackSecret;
};

const toBase64Url = (value: string | Uint8Array) => {
    const base64 = Buffer.from(value).toString('base64');
    return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
};

const fromBase64Url = (value: string) => {
    const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
    return Buffer.from(padded, 'base64').toString('utf-8');
};

const sign = async (payload: string, secret: string) => {
    const key = await crypto.subtle.importKey(
        'raw',
        new TextEncoder().encode(secret),
        { name: 'HMAC', hash: 'SHA-256' },
        false,
        ['sign']
    );

    const signature = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
    return toBase64Url(new Uint8Array(signature));
};

const encodeToken = async (pending: PendingContext, secret: string) => {
    const payloadPart = toBase64Url(JSON.stringify(pending));
    const signaturePart = await sign(payloadPart, secret);
    return `${payloadPart}.${signaturePart}`;
};

const decodeAndVerifyToken = async (token: string, secret: string): Promise<PendingContext | null> => {
    const [payloadPart, signaturePart] = token.split('.');

    if (!payloadPart || !signaturePart) return null;

    const expectedSignature = await sign(payloadPart, secret);
    if (signaturePart !== expectedSignature) return null;

    try {
        const parsed = JSON.parse(fromBase64Url(payloadPart));

        if (
            typeof parsed?.organization_name !== 'string' ||
            typeof parsed?.tenant_id !== 'string' ||
            typeof parsed?.role !== 'string' ||
            parsed?.source !== 'google' ||
            typeof parsed?.created_at !== 'number'
        ) {
            return null;
        }

        return parsed as PendingContext;
    } catch {
        return null;
    }
};

const clearCookie = (response: NextResponse) => {
    response.cookies.set(COOKIE_NAME, '', {
        httpOnly: true,
        secure: process.env.NODE_ENV === 'production',
        sameSite: 'lax',
        path: '/',
        maxAge: 0
    });
};

export async function POST(request: NextRequest) {
    const secret = getSecret();
    if (!secret) {
        return NextResponse.json({ message: 'Secure context secret not configured' }, { status: 503 });
    }

    const body = await request.json().catch(() => ({}));
    const organizationName = String(body?.organizationName || '').trim();
    const tenantId = String(body?.tenantId || body?.organizationId || '').trim();

    if (!organizationName || !tenantId) {
        return NextResponse.json(
            { message: 'organizationName and tenantId are required' },
            { status: 400 }
        );
    }

    const pending: PendingContext = {
        organization_name: organizationName,
        tenant_id: tenantId,
        role: 'master',
        source: 'google',
        created_at: Date.now()
    };

    const token = await encodeToken(pending, secret);

    const response = NextResponse.json({ ok: true });
    response.cookies.set(COOKIE_NAME, token, {
        httpOnly: true,
        secure: process.env.NODE_ENV === 'production',
        sameSite: 'lax',
        path: '/',
        maxAge: MAX_AGE_SECONDS
    });

    return response;
}

export async function GET(request: NextRequest) {
    const secret = getSecret();
    if (!secret) {
        return NextResponse.json({ pending: null });
    }

    const token = request.cookies.get(COOKIE_NAME)?.value;

    if (!token) {
        return NextResponse.json({ pending: null });
    }

    const pending = await decodeAndVerifyToken(token, secret);

    if (!pending || Date.now() - pending.created_at > MAX_AGE_MS) {
        const response = NextResponse.json({ pending: null });
        clearCookie(response);
        return response;
    }

    return NextResponse.json({ pending });
}

export async function DELETE() {
    const response = NextResponse.json({ ok: true });
    clearCookie(response);
    return response;
}