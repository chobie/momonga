// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

type PacketType int

const (
	PACKET_TYPE_RESERVED1   PacketType = 0
	PACKET_TYPE_CONNECT     PacketType = 1
	PACKET_TYPE_CONNACK     PacketType = 2
	PACKET_TYPE_PUBLISH     PacketType = 3
	PACKET_TYPE_PUBACK      PacketType = 4
	PACKET_TYPE_PUBREC      PacketType = 5
	PACKET_TYPE_PUBREL      PacketType = 6
	PACKET_TYPE_PUBCOMP     PacketType = 7
	PACKET_TYPE_SUBSCRIBE   PacketType = 8
	PACKET_TYPE_SUBACK      PacketType = 9
	PACKET_TYPE_UNSUBSCRIBE PacketType = 10
	PACKET_TYPE_UNSUBACK    PacketType = 11
	PACKET_TYPE_PINGREQ     PacketType = 12
	PACKET_TYPE_PINGRESP    PacketType = 13
	PACKET_TYPE_DISCONNECT  PacketType = 14
	PACKET_TYPE_RESERVED2   PacketType = 15
)

type ReturnCode int

const (
	CONNECTION_ACCEOTED                              ReturnCode = 1
	CONNECTION_REFUSED_UNACCEPTABLE_PROTOCOL_VERSION ReturnCode = 2
	CONNECTION_REFUSED_IDENTIFIER_REJECTED           ReturnCode = 3
	CONNECTION_REFUSED_SERVER_UNAVAILABLE            ReturnCode = 4
	CONNECTION_REFUSED_BAD_USER_NAME_OR_PASSWORD     ReturnCode = 5
	CONNECTION_REFUSED_NOT_AUTHORIZED                ReturnCode = 6
)
