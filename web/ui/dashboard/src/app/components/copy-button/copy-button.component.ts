import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { GeneralService } from 'src/app/services/general/general.service';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-copy-button',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './copy-button.component.html',
	styleUrls: ['./copy-button.component.scss']
})
export class CopyButtonComponent implements OnInit {
	@Input('text') textToCopy!: string;
	@Input('notificationText') notificationText!: string;
	@Input('size') size: 'sm' | 'md' = 'sm';
	@Input('className') class!: string;
	@Output('copyText') copy = new EventEmitter();
	textCopied = false;

	constructor(private generalService: GeneralService) {}

	ngOnInit(): void {}

	copyText(event: any) {
		event.stopPropagation();
		if (!this.textToCopy) return;
		const text = this.textToCopy;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		this.textCopied = true;
		this.copy.emit();
		setTimeout(() => {
			this.textCopied = false;
		}, 2000);
		document.body.removeChild(el);
		if (this.notificationText) this.generalService.showNotification({ message: this.notificationText, style: 'info' });
	}
}
