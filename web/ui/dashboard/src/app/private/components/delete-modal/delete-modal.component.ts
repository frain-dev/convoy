import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'app-delete-modal',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './delete-modal.component.html',
	styleUrls: ['./delete-modal.component.scss']
})
export class DeleteModalComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
	@Output() deleteData = new EventEmitter<any>();
	@Input() isLoading: boolean = false;
	@Input() deleteText!: string;
	@Input() deleteButtonText!: string;
	constructor() {}

	ngOnInit(): void {}
}
