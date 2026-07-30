import { Component } from '@angular/core';
import { Router } from '@angular/router';

/**
 * Post-signup holding page (Figma "Verify your email"). Shown right after
 * signup, before the dashboard. The emailed verification link is handled by
 * the separate /verify-email page; this page just prompts the user to check
 * their inbox and lets them continue to the dashboard, which re-checks the
 * verified status itself.
 */
@Component({
	selector: 'convoy-email-verification',
	imports: [],
	templateUrl: './email-verification.component.html',
	styleUrls: ['./email-verification.component.scss']
})
export class EmailVerificationComponent {
	constructor(public router: Router) {}
}
