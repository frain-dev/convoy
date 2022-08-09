import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { StatusColorPipe } from './status-color.pipe';

@NgModule({
	declarations: [StatusColorPipe],
	imports: [CommonModule],
	exports: [StatusColorPipe]
})
export class StatusColorModule {}
