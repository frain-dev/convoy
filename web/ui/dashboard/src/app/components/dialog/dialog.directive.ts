import {CommonModule} from '@angular/common';
import {Component, Directive, ElementRef, EventEmitter, Input, OnDestroy, OnInit, Output} from '@angular/core';

// dialog header
@Component({
    selector: '[convoy-dialog-header]',
    imports: [CommonModule],
    template: `
		<div
		  class="px-24px py-16px border-b border-new.border bg-white-100 w-full"
		  [class.rounded-t-24px]="fullscreen === 'false'"
		  [class.sticky]="fullscreen !== 'false'"
		  [class.top-0]="fullscreen !== 'false'"
		  [class.z-10]="fullscreen !== 'false'">
		  <div class="flex justify-between items-center max-w-[770px] m-auto gap-16px">
		    <div class="flex items-center w-full min-w-0" [ngClass]="{ 'justify-between': fullscreen === 'false' }">
		      <div class="w-full min-w-0" [class]="fullscreen !== 'false' ? 'order-2' : 'order-1'">
		        <ng-content></ng-content>
		      </div>

		      <button
		        type="button"
		        class="w-28px h-28px flex items-center justify-center rounded-full bg-new.surface-muted transition-opacity hover:opacity-80 shrink-0"
		        [class]="fullscreen !== 'false' ? 'order-1 mr-12px' : 'order-2'"
		        (click)="closeDialog.emit()"
		        aria-label="Close">
		        <img src="/assets/img/close-icon.svg" class="w-12px h-12px" alt="" />
		      </button>
		    </div>

		    @if (fullscreen === 'true') {
		      <a
		        target="_blank"
		        href="https://docs.getconvoy.io"
		        rel="noreferrer"
		        class="flex items-center gap-6px font-body font-medium text-[12px] text-new.primary-400 whitespace-nowrap shrink-0 hover:opacity-80">
		        <img src="/assets/img/doc-icon-primary.svg" alt="" class="w-14px h-14px" />
		        Go to docs
		      </a>
		    }
		  </div>
		</div>
		`
})
export class DialogHeaderComponent {
	@Input('fullscreen') fullscreen: 'true' | 'false' | 'custom' = 'false';
	@Output() closeDialog = new EventEmitter();
	constructor() {}
}

@Directive({
	selector: '[convoy-dialog]',
	standalone: true,
	host: { class: 'backdrop:bg-black backdrop:bg-opacity-50 p-0 fixed top-0 left-0 right-0', '[class]': 'classes', '[id]': 'id' }
})
export class DialogDirective implements OnInit, OnDestroy {
	@Input('position') position: 'full' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('id') id!: string;
	/** When `delegate`, Escape is handled by the parent via `(escaped)` instead of closing the dialog. */
	@Input() escapeAction: 'dismiss' | 'delegate' = 'dismiss';
	@Output() escaped = new EventEmitter<void>();

	modalSizes = { sm: 'w-[340px]', md: 'w-[490px]', lg: 'w-[914px]' };
	modalType = {
		full: ` w-full h-full`,
		right: ` mr-0 h-full`,
		center: ` rounded-24px mt-40px`
	};

	private cancelListener?: (event: Event) => void;

	constructor(private el: ElementRef<HTMLDialogElement>) {}

	ngOnInit(): void {
		if (this.escapeAction !== 'delegate') {
			return;
		}

		this.cancelListener = (event: Event) => {
			event.preventDefault();
			this.escaped.emit();
		};
		this.el.nativeElement.addEventListener('cancel', this.cancelListener);
	}

	ngOnDestroy(): void {
		if (this.cancelListener) {
			this.el.nativeElement.removeEventListener('cancel', this.cancelListener);
		}
	}

	get classes(): string {
		return `${this.modalType[this.position]} bg-new.surface-page ${this.position === 'full' ? '' : this.modalSizes[this.size]}`;
	}
}
