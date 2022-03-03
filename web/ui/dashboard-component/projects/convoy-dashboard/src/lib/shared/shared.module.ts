import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { pipes } from './pipes';

@NgModule({
	declarations: [...pipes],
	imports: [CommonModule],
	exports: [...pipes]
})
export class SharedModule {}
