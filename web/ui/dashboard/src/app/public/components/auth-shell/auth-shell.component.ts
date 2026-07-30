import { Component, Input } from '@angular/core';

/**
 * Split-screen shell for the redesigned auth pages (Figma "Auth / Shell").
 * Left: logo, heading, a white card (project with [authCard]) and an optional
 * footer row (project with [authFooter]). Right: brand grid + referral promo,
 * hidden on smaller screens.
 */
@Component({
	selector: 'convoy-auth-shell',
	imports: [],
	templateUrl: './auth-shell.component.html',
	styleUrls: ['./auth-shell.component.scss']
})
export class AuthShellComponent {
	@Input('heading') heading!: string;
	@Input('subheading') subheading?: string;

	// Darker accent cells scattered on the brand grid, echoing the Figma pattern.
	// r = row from the top, k = column offset from the panel's horizontal center.
	accentCells = [
		{ r: 2, k: 6 },
		{ r: 2, k: 7 },
		{ r: 3, k: 8 },
		{ r: 4, k: 7 },
		{ r: 4, k: 8 },
		{ r: 6, k: 4 },
		{ r: 7, k: 5 },
		{ r: 8, k: 4 },
		{ r: 9, k: 5 },
		{ r: 7, k: 11 },
		{ r: 8, k: 10 },
		{ r: 8, k: 11 },
		{ r: 10, k: 2 },
		{ r: 11, k: 3 },
		{ r: 12, k: 1 },
		{ r: 15, k: 5 },
		{ r: 16, k: 6 },
		{ r: 16, k: 7 },
		{ r: 16, k: 8 }
	];

	// 5×5 matrix with radial size falloff (center largest), matching Figma.
	referralDots = this.buildReferralDots();

	cellLeft(k: number): string {
		return `calc(50% + ${k * 36 - 18}px)`;
	}

	private buildReferralDots(): { size: number }[] {
		// Index by Chebyshev distance from center: largest in the middle.
		const sizes = [28, 16, 10];
		const dots: { size: number }[] = [];
		for (let row = 0; row < 5; row++) {
			for (let col = 0; col < 5; col++) {
				const dist = Math.max(Math.abs(row - 2), Math.abs(col - 2));
				dots.push({ size: sizes[Math.min(dist, sizes.length - 1)] });
			}
		}
		return dots;
	}
}
