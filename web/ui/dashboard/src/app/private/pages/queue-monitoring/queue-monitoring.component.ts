import { Component, ElementRef, HostListener, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import axios from 'axios';
import { environment } from 'src/environments/environment';
import { HttpService } from 'src/app/services/http/http.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

@Component({
	selector: 'convoy-queue-monitoring',
	standalone: true,
	imports: [CommonModule, CardComponent, ButtonComponent, LoaderModule],
	templateUrl: './queue-monitoring.component.html',
	styleUrls: ['./queue-monitoring.component.scss']
})
export class QueueMonitoringComponent implements OnInit {
	private static readonly fullscreenStorageKey = 'CONVOY_QUEUE_MONITORING_FULLSCREEN';

	/** Iframe loads here (session cookie path matches this prefix only). */
	embedMonitoringUrl = '';
	/** Main Asynqmon URL (Bearer/Basic only; not used by iframe). */
	directMonitoringUrl = '';

	sessionStatus: 'idle' | 'minting' | 'ready' | 'error' = 'idle';
	sessionError: string | null = null;
	iframeVisible = false;
	iframeFullscreen = false;

	@ViewChild('monitorIframe') monitorIframe?: ElementRef<HTMLIFrameElement>;

	constructor(
		private readonly httpService: HttpService,
		private readonly rbacService: RbacService,
		private readonly router: Router,
		public readonly licenses: LicensesService
	) {}

	async ngOnInit(): Promise<void> {
		const role = await this.rbacService.getUserRole();
		if (role !== 'INSTANCE_ADMIN') {
			this.router.navigateByUrl('/');
			return;
		}

		this.embedMonitoringUrl = this.buildEmbedMonitoringUrl();
		this.directMonitoringUrl = this.buildDirectMonitoringUrl();

		this.loadFullscreenPreference();

		if (this.hasAsynqLicense()) {
			await this.mintSessionAndLoad();
		}
	}

	hasAsynqLicense(): boolean {
		return this.licenses.hasLicense('AsynqMonitoring');
	}

	@HostListener('document:keydown.escape', ['$event'])
	onEscape(): void {
		if (this.iframeFullscreen) {
			this.iframeFullscreen = false;
			this.persistFullscreenPreference();
		}
	}

	toggleIframeFullscreen(): void {
		this.iframeFullscreen = !this.iframeFullscreen;
		this.persistFullscreenPreference();
	}

	async mintSessionAndLoad(): Promise<void> {
		this.sessionStatus = 'minting';
		this.sessionError = null;
		this.iframeVisible = false;

		const token = this.getSessionToken();
		if (!token) {
			this.sessionStatus = 'error';
			this.sessionError = 'No dashboard user token found (log in again).';
			return;
		}

		try {
			const sessionUrl = this.buildSessionUrl();
			await axios.post(sessionUrl, null, {
				headers: {
					Authorization: `Bearer ${token}`,
					'X-Convoy-Version': '2024-04-01'
				},
				withCredentials: true
			});

			this.sessionStatus = 'ready';
			this.iframeVisible = true;

			setTimeout(() => {
				if (this.monitorIframe?.nativeElement) {
					this.monitorIframe.nativeElement.src = this.buildEmbedReloadUrl();
				}
			}, 100);
		} catch (e: unknown) {
			this.sessionStatus = 'error';
			this.sessionError = e instanceof Error ? e.message : String(e);
		}
	}

	openDirectInNewTab(): void {
		window.open(this.directMonitoringUrl, '_blank', 'noopener,noreferrer');
	}

	private getSessionToken(): string | null {
		// Queue monitoring session minting requires the dashboard user token.
		const userToken = this.httpService.authDetails()?.access_token || null;
		if (!userToken) {
			return null;
		}

		return this.normalizeToken(userToken);
	}

	private normalizeToken(token: string): string {
		return token.replace(/^Bearer\s+/i, '').trim();
	}

	private apiBase(): string {
		return environment.production ? location.origin : 'http://localhost:5005';
	}

	private buildEmbedMonitoringUrl(): string {
		return `${this.apiBase()}/queue/monitoring/embed/`;
	}

	private buildEmbedReloadUrl(): string {
		const separator = this.embedMonitoringUrl.includes('?') ? '&' : '?';
		return `${this.embedMonitoringUrl}${separator}_r=${Date.now()}`;
	}

	private buildDirectMonitoringUrl(): string {
		return `${this.apiBase()}/queue/monitoring/`;
	}

	private buildSessionUrl(): string {
		return `${this.apiBase()}/queue/monitoring/session`;
	}

	private loadFullscreenPreference(): void {
		try {
			this.iframeFullscreen = localStorage.getItem(QueueMonitoringComponent.fullscreenStorageKey) === 'true';
		} catch {
			this.iframeFullscreen = false;
		}
	}

	private persistFullscreenPreference(): void {
		try {
			localStorage.setItem(QueueMonitoringComponent.fullscreenStorageKey, this.iframeFullscreen ? 'true' : 'false');
		} catch {
			/* private mode / quota */
		}
	}
}
