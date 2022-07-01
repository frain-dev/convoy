import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';

@Component({
	selector: 'app-delete-modal',
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
