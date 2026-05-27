/**
 * Privacy and content filtering for Co-browsing
 */

interface PrivacyConfig {
	maskInputs?: boolean;           // Mask input fields (default: password fields only)
	maskSelectors?: string[];        // CSS selectors to mask
	allowedDomains?: string[];       // Only allow these domains
	blockDomains?: string[];         // Block these domains
	maskTextPatterns?: RegExp[];    // Mask text matching patterns (e.g., credit cards)
}

let privacyConfig: PrivacyConfig = {};

export function initPrivacy(config: PrivacyConfig): void {
	privacyConfig = {
		maskInputs: true,
		...config
	};
}

export function shouldMaskElement(element: HTMLElement): boolean {
	// Password fields always masked
	if (element instanceof HTMLInputElement && element.type === 'password') {
		return true;
	}

	// Check CSS selectors
	if (privacyConfig.maskSelectors) {
		for (const selector of privacyConfig.maskSelectors) {
			try {
				if (element.matches(selector)) {
					return true;
				}
			} catch (e) {
				console.warn('[Privacy] Invalid selector:', selector);
			}
		}
	}

	return false;
}

export function maskElementValue(element: HTMLElement): string {
	if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
		return '••••••••';
	}
	if (element instanceof HTMLSelectElement) {
		return '••••••••';
	}
	return '••••••••';
}

export function shouldAllowDomain(url: string): boolean {
	try {
		const hostname = new URL(url).hostname;

		// Check block list first
		if (privacyConfig.blockDomains) {
			for (const blocked of privacyConfig.blockDomains) {
				if (hostname === blocked || hostname.endsWith('.' + blocked)) {
					return false;
				}
			}
		}

		// If allow list is configured, check it
		if (privacyConfig.allowedDomains && privacyConfig.allowedDomains.length > 0) {
			for (const allowed of privacyConfig.allowedDomains) {
				if (hostname === allowed || hostname.endsWith('.' + allowed)) {
					return true;
				}
			}
			return false;
		}

		return true;
	} catch {
		return false;
	}
}

export function maskSensitiveText(text: string): string {
	if (!privacyConfig.maskTextPatterns) {
		return text;
	}

	let masked = text;
	for (const pattern of privacyConfig.maskTextPatterns) {
		masked = masked.replace(pattern, '••••');
	}

	return masked;
}

// Common patterns for masking
export const COMMON_PATTERNS = {
	creditCard: /\b(?:\d[ -]*?){13,16}\b/g,
	email: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g,
	phone: /\b\d{3}[-.]?\d{3}[-.]?\d{4}\b/g,
	ssn: /\b\d{3}-\d{2}-\d{4}\b/g,
	ipAddress: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g,
};
