import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-file-input',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './file-input.component.html',
	styleUrls: ['./file-input.component.scss']
})
export class FileInputComponent implements OnInit {
	file: any;
	@Output('selectedFile') selectedFile = new EventEmitter();
	@Output('deleteFile') deleteFile = new EventEmitter();
	constructor() {}

	ngOnInit(): void {}

	parseFile(event: any) {
		this.file = event.target.files[0];
		this.selectedFile.emit(this.file);
	}

	fileSize(size: number) {
		let fileSize;
		if (size < 1000000) fileSize = `${Math.ceil(size / 1000)} kb`;
		else fileSize = `${Math.ceil(size / 1000000)} mb`;

		return fileSize;
	}
}
