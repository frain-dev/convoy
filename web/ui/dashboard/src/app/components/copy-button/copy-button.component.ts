import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { GeneralService } from 'src/app/services/general/general.service';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: '[convoy-copy-button] ,convoy-copy-button',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './copy-button.component.html',
	styleUrls: ['./copy-button.component.scss']
})
export class CopyButtonComponent implements OnInit {
	@Input('text') textToCopy!: string;
	@Input('show-icon') showIcon: 'true' | 'false' = 'true';
	@Input('notificationText') notificationText!: string;
	@Input('size') size: 'sm' | 'md' = 'md';
	@Input('color') color: 'primary' | 'gray' | 'neutral' = 'gray';
	@Input('className') class!: string;
	@Output('copyText') copy = new EventEmitter();
	colors = {
		primary: 'stroke-new.primary-400',
		neutral: 'stroke-neutral-10',
		gray: 'stroke-neutral-10'
	};
	constructor(private generalService: GeneralService) {}

	ngOnInit(): void {}

	async copyItem(event: any) {
		event.stopPropagation();
		if (!this.textToCopy) return;

		try {
			await navigator.clipboard.writeText(this.textToCopy);
			this.copy.emit();
			if (this.notificationText) this.generalService.showNotification({ message: this.notificationText, style: 'info' });
		} catch (err) {
			return err;
		}
	}
}
