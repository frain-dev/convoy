import { ChangeDetectorRef, Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { Subscription } from 'rxjs';
import { NOTIFICATION_STATUS } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-notification',
	imports: [],
	templateUrl: './notification.component.html',
	styleUrls: ['./notification.component.scss']
})
export class NotificationComponent implements OnInit, OnDestroy {
	@ViewChild('toast', { static: true }) toastRef!: ElementRef<HTMLElement>;

	notification: { message: string; style: NOTIFICATION_STATUS; type?: string; show: boolean } = {
		message: '',
		style: 'info',
		type: 'alert',
		show: false
	};

	private sub?: Subscription;

	constructor(
		private generalService: GeneralService,
		private cdr: ChangeDetectorRef
	) {}

	ngOnInit() {
		this.sub = this.generalService.alertStatus.subscribe(res => {
			this.notification = res;
			this.cdr.detectChanges();
			this.syncPopover();
		});
	}

	ngOnDestroy() {
		this.sub?.unsubscribe();
		this.hidePopover();
	}

	dismissNotification() {
		this.generalService.dismissNotification();
	}

	private syncPopover() {
		const el = this.toastRef?.nativeElement;
		if (!el || typeof el.showPopover !== 'function') {
			return;
		}

		const shouldShow = !!(this.notification?.show && this.notification?.type === 'alert');
		if (shouldShow) {
			try {
				el.showPopover();
			} catch {
				/* already open */
			}
		} else {
			this.hidePopover();
		}
	}

	private hidePopover() {
		const el = this.toastRef?.nativeElement;
		if (!el || typeof el.hidePopover !== 'function') {
			return;
		}
		try {
			el.hidePopover();
		} catch {
			/* already closed */
		}
	}
}
