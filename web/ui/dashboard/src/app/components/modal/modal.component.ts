import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-modal',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './modal.component.html',
	styleUrls: ['./modal.component.scss']
})
export class ModalComponent implements OnInit {
	@Input('modalType') modalType = 'side';
	@Input('hasModalHead') hasModalHead = false;
	@Input('hasModalBody') hasModalBody = false;
	@Input('hasModalFooter') hasModalFooter = false;
	constructor() {}

	ngOnInit(): void {}
}
