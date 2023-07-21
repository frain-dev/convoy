import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent, DialogDirective } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-token-modal',
	standalone: true,
	imports: [CommonModule, ModalComponent, CopyButtonComponent, ButtonComponent, DialogDirective],
	templateUrl: './token-modal.component.html',
	styleUrls: ['./token-modal.component.scss']
})
export class TokenModalComponent implements OnInit {
	@ViewChild('tokenDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	@Input('title') title!: string;
	@Input('description') description!: string;
	@Input('token') token!: string;
	@Input('notificationText') notificationText!: string;
	@Output() closeModal = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {
		this.dialog.nativeElement.showModal();
	}

	ngOnDestroy() {
		this.dialog.nativeElement.close();
	}
}
